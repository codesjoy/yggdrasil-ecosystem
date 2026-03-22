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
	"regexp"

	clusterType "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointType "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerType "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

type discoveryEventType int

const (
	listenerAdded discoveryEventType = iota
	routeAdded
	clusterAdded
	endpointAdded
)

type discoveryEvent struct {
	typ  discoveryEventType
	name string
	data any
}

type clusterPolicy struct {
	lbPolicy         string
	maxRequests      uint32
	circuitBreaker   *CircuitBreakerConfig
	outlierDetection *OutlierDetectionConfig
	rateLimiter      *RateLimiterConfig
}

type weightedEndpoint struct {
	endpoint Endpoint
	weight   uint32
	priority uint32
	metadata map[string]string
}

// Endpoint represents a service endpoint.
type Endpoint struct {
	Address string
	Port    int
}

type listenerSnapshot struct {
	route string
}

type routeSnapshot struct {
	vhosts []*VirtualHost
}

type clusterSnapshot struct {
	policy clusterPolicy
}

type edsSnapshot struct {
	endpoints []*weightedEndpoint
}

const (
	typeURLListener = "type.googleapis.com/envoy.config.listener.v3.Listener"
	typeURLRoute    = "type.googleapis.com/envoy.config.route.v3.RouteConfiguration"
	typeURLCluster  = "type.googleapis.com/envoy.config.cluster.v3.Cluster"
	typeURLEndpoint = "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment"
)

func decodeDiscoveryResponse(typeURL string, resources []*anypb.Any) ([]discoveryEvent, error) {
	events := make([]discoveryEvent, 0, len(resources))
	for _, resource := range resources {
		decoded, err := decodeDiscoveryResource(typeURL, resource)
		if err != nil {
			return nil, err
		}
		events = append(events, decoded...)
	}
	return events, nil
}

func decodeDiscoveryResource(typeURL string, resource *anypb.Any) ([]discoveryEvent, error) {
	switch typeURL {
	case typeURLListener:
		listener := &listenerType.Listener{}
		if err := resource.UnmarshalTo(listener); err != nil {
			return nil, fmt.Errorf("failed to unmarshal listener: %w", err)
		}
		return parseListener(listener), nil
	case typeURLRoute:
		route := &routeType.RouteConfiguration{}
		if err := resource.UnmarshalTo(route); err != nil {
			return nil, fmt.Errorf("failed to unmarshal route: %w", err)
		}
		return parseRoute(route), nil
	case typeURLCluster:
		cluster := &clusterType.Cluster{}
		if err := resource.UnmarshalTo(cluster); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cluster: %w", err)
		}
		return parseCluster(cluster), nil
	case typeURLEndpoint:
		endpoint := &endpointType.ClusterLoadAssignment{}
		if err := resource.UnmarshalTo(endpoint); err != nil {
			return nil, fmt.Errorf("failed to unmarshal endpoint: %w", err)
		}
		return parseEndpoint(endpoint), nil
	default:
		return nil, fmt.Errorf("unknown type URL: %s", typeURL)
	}
}

func parseListener(listener *listenerType.Listener) []discoveryEvent {
	if listener == nil || listener.Name == "" {
		return nil
	}

	snapshot := &listenerSnapshot{}
	if hasHTTPConnectionManager(listener) {
		snapshot.route = listener.Name
	}

	return []discoveryEvent{{
		typ:  listenerAdded,
		name: listener.Name,
		data: snapshot,
	}}
}

func hasHTTPConnectionManager(listener *listenerType.Listener) bool {
	for _, filterChain := range listener.FilterChains {
		for _, filter := range filterChain.Filters {
			if filter.Name == "envoy.filters.network.http_connection_manager" {
				return true
			}
		}
	}
	return false
}

func parseRoute(routeConfig *routeType.RouteConfiguration) []discoveryEvent {
	if routeConfig == nil || routeConfig.Name == "" {
		return nil
	}

	snapshot := &routeSnapshot{
		vhosts: make([]*VirtualHost, 0, len(routeConfig.VirtualHosts)),
	}
	for _, virtualHost := range routeConfig.VirtualHosts {
		snapshot.vhosts = append(snapshot.vhosts, parseVirtualHost(virtualHost))
	}

	return []discoveryEvent{{
		typ:  routeAdded,
		name: routeConfig.Name,
		data: snapshot,
	}}
}

func parseVirtualHost(virtualHost *routeType.VirtualHost) *VirtualHost {
	parsed := &VirtualHost{
		Name:    virtualHost.Name,
		Domains: virtualHost.Domains,
		Routes:  make([]*Route, 0, len(virtualHost.Routes)),
	}
	for _, route := range virtualHost.Routes {
		parsed.Routes = append(parsed.Routes, &Route{
			Match:  parseRouteMatch(route.Match),
			Action: parseRouteAction(route.GetRoute()),
		})
	}
	return parsed
}

func parseCluster(cluster *clusterType.Cluster) []discoveryEvent {
	if cluster == nil || cluster.Name == "" {
		return nil
	}

	snapshot := &clusterSnapshot{
		policy: clusterPolicy{
			lbPolicy:    "round_robin",
			maxRequests: 0,
		},
	}

	switch cluster.LbPolicy {
	case clusterType.Cluster_ROUND_ROBIN:
		snapshot.policy.lbPolicy = "round_robin"
	case clusterType.Cluster_RANDOM:
		snapshot.policy.lbPolicy = "random"
	case clusterType.Cluster_LEAST_REQUEST:
		snapshot.policy.lbPolicy = "least_request"
	default:
		snapshot.policy.lbPolicy = "round_robin"
	}

	//nolint:staticcheck // SA1019: MaxRequestsPerConnection is deprecated but still used in older xDS configs
	if cluster.MaxRequestsPerConnection != nil { //nolint:staticcheck
		snapshot.policy.maxRequests = cluster.MaxRequestsPerConnection.Value
	}

	if cluster.CircuitBreakers != nil && len(cluster.CircuitBreakers.Thresholds) > 0 {
		threshold := cluster.CircuitBreakers.Thresholds[0]
		snapshot.policy.circuitBreaker = &CircuitBreakerConfig{
			MaxConnections:     threshold.MaxConnections.GetValue(),
			MaxPendingRequests: threshold.MaxPendingRequests.GetValue(),
			MaxRequests:        threshold.MaxRequests.GetValue(),
			MaxRetries:         threshold.MaxRetries.GetValue(),
		}
	}

	return []discoveryEvent{{
		typ:  clusterAdded,
		name: cluster.Name,
		data: snapshot,
	}}
}

func parseEndpoint(loadAssignment *endpointType.ClusterLoadAssignment) []discoveryEvent {
	if loadAssignment == nil || loadAssignment.ClusterName == "" {
		return nil
	}

	snapshot := &edsSnapshot{
		endpoints: make([]*weightedEndpoint, 0),
	}

	for _, localityEndpoints := range loadAssignment.Endpoints {
		localityWeight := localityEndpoints.GetLoadBalancingWeight().GetValue()
		if localityWeight == 0 {
			localityWeight = 1
		}

		for _, lbEndpoint := range localityEndpoints.LbEndpoints {
			snapshot.endpoints = append(snapshot.endpoints, parseLBEndpoint(
				lbEndpoint,
				localityEndpoints.GetLocality(),
				localityEndpoints.GetPriority(),
				localityWeight,
			))
		}
	}

	return []discoveryEvent{{
		typ:  endpointAdded,
		name: loadAssignment.ClusterName,
		data: snapshot,
	}}
}

func parseLBEndpoint(
	lbEndpoint *endpointType.LbEndpoint,
	locality *corev3.Locality,
	priority uint32,
	localityWeight uint32,
) *weightedEndpoint {
	weight := lbEndpoint.GetLoadBalancingWeight().GetValue()
	if weight == 0 {
		weight = 1
	}

	endpoint := Endpoint{}
	if lbEndpoint.HostIdentifier != nil {
		if address := lbEndpoint.GetEndpoint().GetAddress(); address != nil {
			socketAddress := address.GetSocketAddress()
			endpoint.Address = socketAddress.GetAddress()
			endpoint.Port = int(socketAddress.GetPortValue())
		}
	}

	return &weightedEndpoint{
		endpoint: endpoint,
		weight:   weight * localityWeight,
		priority: priority,
		metadata: parseEndpointMetadata(locality, lbEndpoint.GetHealthStatus()),
	}
}

func parseEndpointMetadata(
	locality *corev3.Locality,
	healthStatus corev3.HealthStatus,
) map[string]string {
	metadata := make(map[string]string)
	if locality != nil {
		if locality.Region != "" {
			metadata["region"] = locality.Region
		}
		if locality.Zone != "" {
			metadata["zone"] = locality.Zone
		}
		if locality.SubZone != "" {
			metadata["sub_zone"] = locality.SubZone
		}
	}
	metadata["health"] = endpointHealthStatus(healthStatus)
	return metadata
}

func endpointHealthStatus(status corev3.HealthStatus) string {
	switch status {
	case 0:
		return "HEALTHY"
	case 1:
		return "UNHEALTHY"
	case 2:
		return "DRAINING"
	case 3:
		return "TIMEOUT"
	case 4:
		return "DEGRADED"
	default:
		return "UNKNOWN"
	}
}

func parseRouteMatch(match *routeType.RouteMatch) *RouteMatch {
	if match == nil {
		return nil
	}

	parsed := &RouteMatch{}
	if match.PathSpecifier != nil {
		switch pathSpecifier := match.PathSpecifier.(type) {
		case *routeType.RouteMatch_Prefix:
			parsed.Prefix = pathSpecifier.Prefix
		case *routeType.RouteMatch_Path:
			parsed.Path = pathSpecifier.Path
		case *routeType.RouteMatch_SafeRegex:
			if pathSpecifier.SafeRegex != nil && pathSpecifier.SafeRegex.Regex != "" {
				if compiled, err := regexp.Compile(pathSpecifier.SafeRegex.Regex); err == nil {
					parsed.Regex = compiled
				}
			}
		}
	}

	for _, header := range match.Headers {
		headerMatcher := &HeaderMatcher{Name: header.Name}
		switch specifier := header.HeaderMatchSpecifier.(type) {
		case *routeType.HeaderMatcher_ExactMatch:
			headerMatcher.ExactMatch = specifier.ExactMatch //nolint:staticcheck
		case *routeType.HeaderMatcher_SafeRegexMatch:
			if specifier.SafeRegexMatch != nil && specifier.SafeRegexMatch.Regex != "" { //nolint:staticcheck
				if compiled, err := regexp.Compile(specifier.SafeRegexMatch.Regex); err == nil { //nolint:staticcheck
					headerMatcher.RegexMatch = compiled
				}
			}
		case *routeType.HeaderMatcher_PresentMatch:
			headerMatcher.Present = specifier.PresentMatch
		case *routeType.HeaderMatcher_PrefixMatch:
			headerMatcher.PrefixMatch = specifier.PrefixMatch //nolint:staticcheck
		case *routeType.HeaderMatcher_SuffixMatch:
			headerMatcher.SuffixMatch = specifier.SuffixMatch //nolint:staticcheck
		}
		parsed.Headers = append(parsed.Headers, headerMatcher)
	}

	return parsed
}

func parseRouteAction(action *routeType.RouteAction) *RouteAction {
	if action == nil {
		return nil
	}

	parsed := &RouteAction{}
	switch clusterSpecifier := action.ClusterSpecifier.(type) {
	case *routeType.RouteAction_Cluster:
		parsed.Cluster = clusterSpecifier.Cluster
	case *routeType.RouteAction_WeightedClusters:
		if clusterSpecifier.WeightedClusters == nil {
			return parsed
		}

		weighted := &WeightedClusters{}
		if clusterSpecifier.WeightedClusters.TotalWeight != nil { //nolint:staticcheck
			weighted.TotalWeight = clusterSpecifier.WeightedClusters.TotalWeight.Value //nolint:staticcheck
		}
		for _, cluster := range clusterSpecifier.WeightedClusters.Clusters {
			weight := uint32(0)
			if cluster.Weight != nil {
				weight = cluster.Weight.Value
			}
			weighted.Clusters = append(weighted.Clusters, &WeightedCluster{
				Name:   cluster.Name,
				Weight: weight,
			})
		}
		parsed.WeightedClusters = weighted
	}

	return parsed
}
