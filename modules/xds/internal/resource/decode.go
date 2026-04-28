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

package resource

import (
	"fmt"
	"regexp"
	"time"

	clusterType "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointType "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerType "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcmType "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	typeURLListener = "type.googleapis.com/envoy.config.listener.v3.Listener"
	typeURLRoute    = "type.googleapis.com/envoy.config.route.v3.RouteConfiguration"
	typeURLCluster  = "type.googleapis.com/envoy.config.cluster.v3.Cluster"
	typeURLEndpoint = "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment"

	httpConnectionManagerFilter = "envoy.filters.network.http_connection_manager"
	rateLimitMetadataKey        = "yggdrasil.rate_limit"
)

// DecodeDiscoveryResponse decodes a DiscoveryResponse resource list into events.
func DecodeDiscoveryResponse(typeURL string, resources []*anypb.Any) ([]DiscoveryEvent, error) {
	events := make([]DiscoveryEvent, 0, len(resources))
	for _, item := range resources {
		decoded, err := DecodeDiscoveryResource(typeURL, item)
		if err != nil {
			return nil, err
		}
		events = append(events, decoded...)
	}
	return events, nil
}

// DecodeDiscoveryResource decodes one xDS resource into events.
func DecodeDiscoveryResource(typeURL string, item *anypb.Any) ([]DiscoveryEvent, error) {
	switch typeURL {
	case typeURLListener:
		resource := &listenerType.Listener{}
		if err := item.UnmarshalTo(resource); err != nil {
			return nil, fmt.Errorf("unmarshal listener: %w", err)
		}
		return parseListener(resource), nil
	case typeURLRoute:
		resource := &routeType.RouteConfiguration{}
		if err := item.UnmarshalTo(resource); err != nil {
			return nil, fmt.Errorf("unmarshal route: %w", err)
		}
		return parseRoute(resource), nil
	case typeURLCluster:
		resource := &clusterType.Cluster{}
		if err := item.UnmarshalTo(resource); err != nil {
			return nil, fmt.Errorf("unmarshal cluster: %w", err)
		}
		return parseCluster(resource), nil
	case typeURLEndpoint:
		resource := &endpointType.ClusterLoadAssignment{}
		if err := item.UnmarshalTo(resource); err != nil {
			return nil, fmt.Errorf("unmarshal endpoint: %w", err)
		}
		return parseEndpoint(resource), nil
	default:
		return nil, fmt.Errorf("unknown type URL: %s", typeURL)
	}
}

func parseListener(listener *listenerType.Listener) []DiscoveryEvent {
	if listener == nil || listener.Name == "" {
		return nil
	}

	return []DiscoveryEvent{{
		Typ:  ListenerAdded,
		Name: listener.Name,
		Data: &ListenerSnapshot{Route: routeNameForListener(listener)},
	}}
}

func routeNameForListener(listener *listenerType.Listener) string {
	for _, filterChain := range listener.FilterChains {
		for _, filter := range filterChain.Filters {
			if filter.Name != httpConnectionManagerFilter {
				continue
			}

			manager := &hcmType.HttpConnectionManager{}
			if typed := filter.GetTypedConfig(); typed != nil && typed.UnmarshalTo(manager) == nil {
				switch specifier := manager.RouteSpecifier.(type) {
				case *hcmType.HttpConnectionManager_Rds:
					if specifier.Rds != nil && specifier.Rds.RouteConfigName != "" {
						return specifier.Rds.RouteConfigName
					}
				case *hcmType.HttpConnectionManager_RouteConfig:
					if specifier.RouteConfig != nil && specifier.RouteConfig.Name != "" {
						return specifier.RouteConfig.Name
					}
				}
			}

			return listener.Name
		}
	}

	return ""
}

func parseRoute(routeConfig *routeType.RouteConfiguration) []DiscoveryEvent {
	if routeConfig == nil || routeConfig.Name == "" {
		return nil
	}

	snapshot := &RouteSnapshot{
		Vhosts: make([]*VirtualHost, 0, len(routeConfig.VirtualHosts)),
	}
	for _, virtualHost := range routeConfig.VirtualHosts {
		snapshot.Vhosts = append(snapshot.Vhosts, parseVirtualHost(virtualHost))
	}

	return []DiscoveryEvent{{
		Typ:  RouteAdded,
		Name: routeConfig.Name,
		Data: snapshot,
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

func parseCluster(cluster *clusterType.Cluster) []DiscoveryEvent {
	if cluster == nil || cluster.Name == "" {
		return nil
	}

	snapshot := &ClusterSnapshot{
		Policy: ClusterPolicy{
			LBPolicy: "round_robin",
		},
	}

	switch cluster.LbPolicy {
	case clusterType.Cluster_RANDOM:
		snapshot.Policy.LBPolicy = "random"
	case clusterType.Cluster_LEAST_REQUEST:
		snapshot.Policy.LBPolicy = "least_request"
	default:
		snapshot.Policy.LBPolicy = "round_robin"
	}

	//nolint:staticcheck // SA1019: deprecated but still used in older xDS configs.
	if cluster.MaxRequestsPerConnection != nil { //nolint:staticcheck
		snapshot.Policy.MaxRequests = cluster.MaxRequestsPerConnection.Value
	}

	if cluster.CircuitBreakers != nil && len(cluster.CircuitBreakers.Thresholds) > 0 {
		threshold := cluster.CircuitBreakers.Thresholds[0]
		snapshot.Policy.CircuitBreaker = &CircuitBreakerConfig{
			MaxConnections:     threshold.GetMaxConnections().GetValue(),
			MaxPendingRequests: threshold.GetMaxPendingRequests().GetValue(),
			MaxRequests:        threshold.GetMaxRequests().GetValue(),
			MaxRetries:         threshold.GetMaxRetries().GetValue(),
		}
	}

	if cluster.OutlierDetection != nil {
		outlier := cluster.OutlierDetection
		snapshot.Policy.OutlierDetection = &OutlierDetectionConfig{
			Consecutive5xx:                 outlier.GetConsecutive_5Xx().GetValue(),
			ConsecutiveGatewayFailure:      outlier.GetConsecutiveGatewayFailure().GetValue(),
			ConsecutiveLocalOriginFailure:  outlier.GetConsecutiveLocalOriginFailure().GetValue(),
			Interval:                       outlier.GetInterval().AsDuration(),
			BaseEjectionTime:               outlier.GetBaseEjectionTime().AsDuration(),
			MaxEjectionTime:                outlier.GetMaxEjectionTime().AsDuration(),
			MaxEjectionPercent:             outlier.GetMaxEjectionPercent().GetValue(),
			EnforcingConsecutive5xx:        outlier.GetEnforcingConsecutive_5Xx().GetValue(),
			EnforcingSuccessRate:           outlier.GetEnforcingSuccessRate().GetValue(),
			SuccessRateMinimumHosts:        outlier.GetSuccessRateMinimumHosts().GetValue(),
			SuccessRateRequestVolume:       outlier.GetSuccessRateRequestVolume().GetValue(),
			SuccessRateStdevFactor:         outlier.GetSuccessRateStdevFactor().GetValue(),
			FailurePercentageThreshold:     outlier.GetFailurePercentageThreshold().GetValue(),
			EnforcingFailurePercentage:     outlier.GetEnforcingFailurePercentage().GetValue(),
			FailurePercentageMinimumHosts:  outlier.GetFailurePercentageMinimumHosts().GetValue(),
			FailurePercentageRequestVolume: outlier.GetFailurePercentageRequestVolume().GetValue(),
			SplitExternalLocalOriginErrors: outlier.SplitExternalLocalOriginErrors,
		}
	}

	if limiter := parseRateLimiter(cluster.Metadata); limiter != nil {
		snapshot.Policy.RateLimiter = limiter
	}

	return []DiscoveryEvent{{
		Typ:  ClusterAdded,
		Name: cluster.Name,
		Data: snapshot,
	}}
}

func parseRateLimiter(metadata *corev3.Metadata) *RateLimiterConfig {
	if metadata == nil || metadata.FilterMetadata == nil {
		return nil
	}

	config := metadata.FilterMetadata[rateLimitMetadataKey]
	if config == nil {
		return nil
	}

	fields := config.GetFields()
	if len(fields) == 0 {
		return nil
	}

	return &RateLimiterConfig{
		MaxTokens:     uint32(numberValue(fields["max_tokens"])),
		TokensPerFill: uint32(numberValue(fields["tokens_per_fill"])),
		FillInterval:  time.Duration(numberValue(fields["fill_interval"]) * float64(time.Second)),
	}
}

func numberValue(value *structpb.Value) float64 {
	if value == nil {
		return 0
	}
	return value.GetNumberValue()
}

func parseEndpoint(loadAssignment *endpointType.ClusterLoadAssignment) []DiscoveryEvent {
	if loadAssignment == nil || loadAssignment.ClusterName == "" {
		return nil
	}

	snapshot := &EDSSnapshot{
		Endpoints: make([]*WeightedEndpoint, 0),
	}

	for _, localityEndpoints := range loadAssignment.Endpoints {
		localityWeight := localityEndpoints.GetLoadBalancingWeight().GetValue()
		if localityWeight == 0 {
			localityWeight = 1
		}

		for _, lbEndpoint := range localityEndpoints.LbEndpoints {
			snapshot.Endpoints = append(snapshot.Endpoints, parseLBEndpoint(
				loadAssignment.ClusterName,
				lbEndpoint,
				localityEndpoints.GetLocality(),
				localityEndpoints.GetPriority(),
				localityWeight,
			))
		}
	}

	return []DiscoveryEvent{{
		Typ:  EndpointAdded,
		Name: loadAssignment.ClusterName,
		Data: snapshot,
	}}
}

func parseLBEndpoint(
	clusterName string,
	lbEndpoint *endpointType.LbEndpoint,
	locality *corev3.Locality,
	priority uint32,
	localityWeight uint32,
) *WeightedEndpoint {
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

	return &WeightedEndpoint{
		Cluster:  clusterName,
		Endpoint: endpoint,
		Weight:   weight * localityWeight,
		Priority: priority,
		Metadata: parseEndpointMetadata(locality, lbEndpoint.GetHealthStatus()),
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
