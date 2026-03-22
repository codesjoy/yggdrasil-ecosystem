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

package xds

import (
	"fmt"

	"github.com/codesjoy/yggdrasil/v2/resolver"
)

func (c *xdsCore) handleDiscoveryEvent(event discoveryEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch event.typ {
	case listenerAdded:
		snapshot := event.data.(*listenerSnapshot)
		c.listeners[event.name] = snapshot
		c.trackDependencies(event.name, snapshot)
	case routeAdded:
		snapshot := event.data.(*routeSnapshot)
		c.routes[event.name] = snapshot
		c.updateRouteDependencies(snapshot)
	case clusterAdded:
		c.clusters[event.name] = event.data.(*clusterSnapshot)
	case endpointAdded:
		c.endpoints[event.name] = event.data.(*edsSnapshot)
	}

	c.reconcileSubscriptions()
	c.notifyApps()
}

func (c *xdsCore) trackDependencies(listenerName string, snapshot *listenerSnapshot) {
	if snapshot == nil {
		return
	}

	dependency := c.ensureDependenciesLocked(listenerName)
	if snapshot.route != "" {
		dependency.routes[snapshot.route] = true
	}
}

func (c *xdsCore) ensureDependenciesLocked(listenerName string) *resourceDependencies {
	if dependency, ok := c.dependencies[listenerName]; ok {
		return dependency
	}

	dependency := &resourceDependencies{
		routes:   make(map[string]bool),
		clusters: make(map[string]bool),
	}
	c.dependencies[listenerName] = dependency
	return dependency
}

func (c *xdsCore) updateRouteDependencies(snapshot *routeSnapshot) {
	if snapshot == nil {
		return
	}

	for _, clusterName := range routeClusterNames(snapshot) {
		for _, dependency := range c.dependencies {
			dependency.clusters[clusterName] = true
		}
	}
}

func (c *xdsCore) reconcileSubscriptions() {
	ldsNames, rdsNames, cdsNames := c.collectSubscriptionNames()
	edsNames := append([]string(nil), cdsNames...)

	if c.ads != nil {
		c.ads.UpdateSubscriptions(ldsNames, rdsNames, cdsNames, edsNames)
	}
}

func (c *xdsCore) collectSubscriptionNames() (ldsNames, rdsNames, cdsNames []string) {
	for _, app := range c.apps {
		for listenerName := range app.listeners {
			ldsNames = append(ldsNames, listenerName)

			routeName, ok := c.routeNameForListener(listenerName)
			if ok {
				rdsNames = append(rdsNames, routeName)
			}
			if routeSnapshot, ok := c.routes[routeName]; ok {
				cdsNames = append(cdsNames, routeClusterNames(routeSnapshot)...)
			}

			if dependency, ok := c.dependencies[listenerName]; ok {
				for clusterName := range dependency.clusters {
					if !contains(cdsNames, clusterName) {
						cdsNames = append(cdsNames, clusterName)
					}
				}
			}
		}

		for clusterName := range app.clusters {
			cdsNames = append(cdsNames, clusterName)
		}
	}

	return ldsNames, rdsNames, cdsNames
}

func routeClusterNames(snapshot *routeSnapshot) []string {
	if snapshot == nil {
		return nil
	}

	clusterNames := make([]string, 0)
	for _, virtualHost := range snapshot.vhosts {
		for _, route := range virtualHost.Routes {
			if route.Action == nil {
				continue
			}
			if route.Action.Cluster != "" {
				clusterNames = append(clusterNames, route.Action.Cluster)
			}
			if route.Action.WeightedClusters != nil {
				for _, weightedCluster := range route.Action.WeightedClusters.Clusters {
					clusterNames = append(clusterNames, weightedCluster.Name)
				}
			}
		}
	}

	return clusterNames
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (c *xdsCore) notifyApps() {
	for appName, app := range c.apps {
		endpoints := c.buildResolverEndpoints(app)
		state := resolver.BaseState{
			Endpoints:  endpoints,
			Attributes: c.buildResolverAttributes(app),
		}
		if c.onUpdate != nil {
			c.onUpdate(appName, state)
		}
	}
}

func (c *xdsCore) buildResolverEndpoints(app *appInfo) []resolver.Endpoint {
	weightedEndpoints := c.collectAppEndpoints(app)
	endpoints := make([]resolver.Endpoint, 0, len(weightedEndpoints))
	for _, endpoint := range weightedEndpoints {
		endpoints = append(endpoints, resolver.BaseEndpoint{
			Address:  fmt.Sprintf("%s:%d", endpoint.endpoint.Address, endpoint.endpoint.Port),
			Protocol: c.cfg.Protocol,
			Attributes: map[string]any{
				"weight":   endpoint.weight,
				"priority": endpoint.priority,
				"metadata": endpoint.metadata,
			},
		})
	}
	return endpoints
}

func (c *xdsCore) collectAppEndpoints(app *appInfo) []*weightedEndpoint {
	var endpoints []*weightedEndpoint
	for listenerName := range app.listeners {
		routeName, ok := c.routeNameForListener(listenerName)
		if !ok {
			continue
		}

		routeSnapshot, ok := c.routes[routeName]
		if !ok {
			continue
		}

		clusters := make(map[string]bool)
		for _, clusterName := range routeClusterNames(routeSnapshot) {
			clusters[clusterName] = true
		}
		for clusterName := range clusters {
			endpointSnapshot, ok := c.endpoints[clusterName]
			if !ok {
				continue
			}
			for _, endpoint := range endpointSnapshot.endpoints {
				endpoints = append(endpoints, copyWeightedEndpoint(endpoint))
			}
		}
	}
	return endpoints
}

func (c *xdsCore) routeNameForListener(listenerName string) (string, bool) {
	listenerSnapshot, ok := c.listeners[listenerName]
	if !ok || listenerSnapshot.route == "" {
		return "", false
	}
	return listenerSnapshot.route, true
}

func copyWeightedEndpoint(endpoint *weightedEndpoint) *weightedEndpoint {
	return &weightedEndpoint{
		endpoint: endpoint.endpoint,
		weight:   endpoint.weight,
		priority: endpoint.priority,
		metadata: endpoint.metadata,
	}
}

func (c *xdsCore) buildResolverAttributes(app *appInfo) map[string]any {
	return map[string]any{
		"xds_routes":   buildRouteConfig(app, c.routes, c.listeners),
		"xds_clusters": buildClusterMap(app),
	}
}

func buildRouteConfig(
	app *appInfo,
	routes map[string]*routeSnapshot,
	listeners map[string]*listenerSnapshot,
) []*VirtualHost {
	var vhosts []*VirtualHost
	for listenerName := range app.listeners {
		listenerSnapshot, ok := listeners[listenerName]
		if !ok {
			continue
		}
		routeSnapshot, ok := routes[listenerSnapshot.route]
		if !ok {
			continue
		}
		vhosts = append(vhosts, routeSnapshot.vhosts...)
	}
	return vhosts
}

func buildClusterMap(app *appInfo) map[string]clusterPolicy {
	clusters := make(map[string]clusterPolicy)
	for clusterName := range app.clusters {
		clusters[clusterName] = clusterPolicy{}
	}
	return clusters
}
