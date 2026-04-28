// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolver

import (
	"fmt"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

func (c *xdsCore) reconcileSubscriptions() {
	ldsNames, rdsNames, cdsNames := c.collectSubscriptionNames()
	edsNames := append([]string(nil), cdsNames...)

	if c.ads != nil {
		c.ads.UpdateSubscriptions(ldsNames, rdsNames, cdsNames, edsNames)
	}
}

func (c *xdsCore) collectSubscriptionNames() (ldsNames, rdsNames, cdsNames []string) {
	ldsSet := make(map[string]struct{})
	rdsSet := make(map[string]struct{})
	cdsSet := make(map[string]struct{})

	for _, app := range c.apps {
		for listenerName := range app.listeners {
			ldsSet[listenerName] = struct{}{}

			routeName, ok := c.routeNameForListener(listenerName)
			if !ok {
				continue
			}

			rdsSet[routeName] = struct{}{}
			for _, clusterName := range routeClusterNames(c.routes[routeName]) {
				cdsSet[clusterName] = struct{}{}
			}
		}
	}

	return setKeys(ldsSet), setKeys(rdsSet), setKeys(cdsSet)
}

func setKeys(input map[string]struct{}) []string {
	out := make([]string, 0, len(input))
	for item := range input {
		out = append(out, item)
	}
	return out
}

func routeClusterNames(snapshot *xdsresource.RouteSnapshot) []string {
	if snapshot == nil {
		return nil
	}

	set := make(map[string]struct{})
	for _, virtualHost := range snapshot.Vhosts {
		for _, route := range virtualHost.Routes {
			if route.Action == nil {
				continue
			}
			if route.Action.Cluster != "" {
				set[route.Action.Cluster] = struct{}{}
			}
			if route.Action.WeightedClusters != nil {
				for _, weightedCluster := range route.Action.WeightedClusters.Clusters {
					set[weightedCluster.Name] = struct{}{}
				}
			}
		}
	}

	return setKeys(set)
}

func (c *xdsCore) notifyApps() {
	for appName, app := range c.apps {
		endpoints := c.buildResolverEndpoints(app)
		state := yresolver.BaseState{
			Endpoints:  endpoints,
			Attributes: c.buildResolverAttributes(app),
		}
		if c.onUpdate != nil {
			c.onUpdate(appName, state)
		}
	}
}

func (c *xdsCore) buildResolverEndpoints(app *appInfo) []yresolver.Endpoint {
	weightedEndpoints := c.collectAppEndpoints(app)
	endpoints := make([]yresolver.Endpoint, 0, len(weightedEndpoints))
	for _, endpoint := range weightedEndpoints {
		endpoints = append(endpoints, yresolver.BaseEndpoint{
			Address:  fmt.Sprintf("%s:%d", endpoint.Endpoint.Address, endpoint.Endpoint.Port),
			Protocol: c.cfg.Protocol,
			Attributes: map[string]any{
				xdsresource.AttributeEndpointCluster:  endpoint.Cluster,
				xdsresource.AttributeEndpointWeight:   endpoint.Weight,
				xdsresource.AttributeEndpointPriority: endpoint.Priority,
				xdsresource.AttributeEndpointMetadata: endpoint.Metadata,
			},
		})
	}
	return endpoints
}

func (c *xdsCore) collectAppEndpoints(app *appInfo) []*xdsresource.WeightedEndpoint {
	var endpoints []*xdsresource.WeightedEndpoint
	seen := make(map[string]struct{})

	for clusterName := range c.clusterNamesForApp(app) {
		endpointSnapshot, ok := c.endpoints[clusterName]
		if !ok {
			continue
		}
		for _, endpoint := range endpointSnapshot.Endpoints {
			key := fmt.Sprintf(
				"%s|%s:%d",
				clusterName,
				endpoint.Endpoint.Address,
				endpoint.Endpoint.Port,
			)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			endpoints = append(endpoints, copyWeightedEndpoint(endpoint))
		}
	}

	return endpoints
}

func (c *xdsCore) routeNameForListener(listenerName string) (string, bool) {
	listenerSnapshot, ok := c.listeners[listenerName]
	if !ok || listenerSnapshot.Route == "" {
		return "", false
	}
	return listenerSnapshot.Route, true
}

func copyWeightedEndpoint(endpoint *xdsresource.WeightedEndpoint) *xdsresource.WeightedEndpoint {
	return &xdsresource.WeightedEndpoint{
		Cluster:  endpoint.Cluster,
		Endpoint: endpoint.Endpoint,
		Weight:   endpoint.Weight,
		Priority: endpoint.Priority,
		Metadata: endpoint.Metadata,
	}
}

func (c *xdsCore) buildResolverAttributes(app *appInfo) map[string]any {
	return map[string]any{
		xdsresource.AttributeRoutes:   buildRouteConfig(app, c.routes, c.listeners),
		xdsresource.AttributeClusters: buildClusterMap(app, c.routes, c.listeners, c.clusters),
	}
}

func buildRouteConfig(
	app *appInfo,
	routes map[string]*xdsresource.RouteSnapshot,
	listeners map[string]*xdsresource.ListenerSnapshot,
) []*xdsresource.VirtualHost {
	var vhosts []*xdsresource.VirtualHost
	for listenerName := range app.listeners {
		listenerSnapshot, ok := listeners[listenerName]
		if !ok {
			continue
		}
		routeSnapshot, ok := routes[listenerSnapshot.Route]
		if !ok {
			continue
		}
		vhosts = append(vhosts, routeSnapshot.Vhosts...)
	}
	return vhosts
}

func buildClusterMap(
	app *appInfo,
	routes map[string]*xdsresource.RouteSnapshot,
	listeners map[string]*xdsresource.ListenerSnapshot,
	clusters map[string]*xdsresource.ClusterSnapshot,
) map[string]xdsresource.ClusterPolicy {
	clusterPolicies := make(map[string]xdsresource.ClusterPolicy)
	for clusterName := range clusterNamesForApp(app, routes, listeners) {
		policy := xdsresource.ClusterPolicy{}
		if snapshot := clusters[clusterName]; snapshot != nil {
			policy = snapshot.Policy
		}
		clusterPolicies[clusterName] = policy
	}
	return clusterPolicies
}

func (c *xdsCore) clusterNamesForApp(app *appInfo) map[string]struct{} {
	return clusterNamesForApp(app, c.routes, c.listeners)
}

func clusterNamesForApp(
	app *appInfo,
	routes map[string]*xdsresource.RouteSnapshot,
	listeners map[string]*xdsresource.ListenerSnapshot,
) map[string]struct{} {
	clusterNames := make(map[string]struct{})
	for listenerName := range app.listeners {
		listenerSnapshot, ok := listeners[listenerName]
		if !ok {
			continue
		}
		routeSnapshot, ok := routes[listenerSnapshot.Route]
		if !ok {
			continue
		}
		for _, clusterName := range routeClusterNames(routeSnapshot) {
			clusterNames[clusterName] = struct{}{}
		}
	}
	return clusterNames
}
