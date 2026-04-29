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
	"testing"
	"time"

	clusterType "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointType "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerType "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcmType "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	matcherType "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestListenerParsingFallbacks(t *testing.T) {
	if got := parseListener(nil); got != nil {
		t.Fatalf("parseListener(nil) = %#v, want nil", got)
	}
	if got := parseListener(&listenerType.Listener{}); got != nil {
		t.Fatalf("parseListener(empty) = %#v, want nil", got)
	}

	inlineAny, err := anypb.New(&hcmType.HttpConnectionManager{
		RouteSpecifier: &hcmType.HttpConnectionManager_RouteConfig{
			RouteConfig: &routeType.RouteConfiguration{Name: "inline-route"},
		},
	})
	if err != nil {
		t.Fatalf("anypb.New() error = %v", err)
	}

	inlineListener := &listenerType.Listener{
		Name: "listener-inline",
		FilterChains: []*listenerType.FilterChain{{
			Filters: []*listenerType.Filter{{
				Name:       httpConnectionManagerFilter,
				ConfigType: &listenerType.Filter_TypedConfig{TypedConfig: inlineAny},
			}},
		}},
	}
	if got := routeNameForListener(inlineListener); got != "inline-route" {
		t.Fatalf("routeNameForListener(inline) = %q, want inline-route", got)
	}

	badListener := &listenerType.Listener{
		Name: "listener-bad",
		FilterChains: []*listenerType.FilterChain{{
			Filters: []*listenerType.Filter{{
				Name: httpConnectionManagerFilter,
				ConfigType: &listenerType.Filter_TypedConfig{
					TypedConfig: &anypb.Any{TypeUrl: "bad", Value: []byte("bad")},
				},
			}},
		}},
	}
	if got := routeNameForListener(badListener); got != "listener-bad" {
		t.Fatalf("routeNameForListener(bad config) = %q, want listener-bad", got)
	}

	noHTTP := &listenerType.Listener{
		Name: "listener-empty",
		FilterChains: []*listenerType.FilterChain{{
			Filters: []*listenerType.Filter{{Name: "envoy.filters.network.tcp_proxy"}},
		}},
	}
	if got := routeNameForListener(noHTTP); got != "" {
		t.Fatalf("routeNameForListener(no http filter) = %q, want empty", got)
	}
}

func TestClusterAndEndpointParsingEdges(t *testing.T) {
	if got := parseCluster(nil); got != nil {
		t.Fatalf("parseCluster(nil) = %#v, want nil", got)
	}
	if got := parseCluster(&clusterType.Cluster{}); got != nil {
		t.Fatalf("parseCluster(empty) = %#v, want nil", got)
	}

	cluster := &clusterType.Cluster{
		Name:                     "cluster-random",
		LbPolicy:                 clusterType.Cluster_RANDOM,
		MaxRequestsPerConnection: wrapperspb.UInt32(12),
		CircuitBreakers: &clusterType.CircuitBreakers{
			Thresholds: []*clusterType.CircuitBreakers_Thresholds{{
				MaxConnections:     wrapperspb.UInt32(1),
				MaxPendingRequests: wrapperspb.UInt32(2),
				MaxRequests:        wrapperspb.UInt32(3),
				MaxRetries:         wrapperspb.UInt32(4),
			}},
		},
		Metadata: &corev3.Metadata{
			FilterMetadata: map[string]*structpb.Struct{
				rateLimitMetadataKey: {
					Fields: map[string]*structpb.Value{},
				},
			},
		},
	}
	events := parseCluster(cluster)
	snapshot := events[0].Data.(*ClusterSnapshot)
	if snapshot.Policy.LBPolicy != "random" {
		t.Fatalf("LBPolicy = %q, want random", snapshot.Policy.LBPolicy)
	}
	if snapshot.Policy.MaxRequests != 12 {
		t.Fatalf("MaxRequests = %d, want 12", snapshot.Policy.MaxRequests)
	}
	if snapshot.Policy.CircuitBreaker == nil || snapshot.Policy.CircuitBreaker.MaxRetries != 4 {
		t.Fatalf("CircuitBreaker = %#v", snapshot.Policy.CircuitBreaker)
	}
	if snapshot.Policy.RateLimiter != nil {
		t.Fatalf("RateLimiter = %#v, want nil for empty metadata", snapshot.Policy.RateLimiter)
	}

	defaultCluster := parseCluster(&clusterType.Cluster{Name: "cluster-default"})
	if got := defaultCluster[0].Data.(*ClusterSnapshot).Policy.LBPolicy; got != "round_robin" {
		t.Fatalf("default LBPolicy = %q, want round_robin", got)
	}

	if got := parseRateLimiter(nil); got != nil {
		t.Fatalf("parseRateLimiter(nil) = %#v, want nil", got)
	}
	if got := parseRateLimiter(&corev3.Metadata{
		FilterMetadata: map[string]*structpb.Struct{rateLimitMetadataKey: {Fields: map[string]*structpb.Value{}}},
	}); got != nil {
		t.Fatalf("parseRateLimiter(empty fields) = %#v, want nil", got)
	}
	if got := numberValue(nil); got != 0 {
		t.Fatalf("numberValue(nil) = %v, want 0", got)
	}

	if got := parseEndpoint(nil); got != nil {
		t.Fatalf("parseEndpoint(nil) = %#v, want nil", got)
	}
	if got := parseEndpoint(&endpointType.ClusterLoadAssignment{}); got != nil {
		t.Fatalf("parseEndpoint(empty) = %#v, want nil", got)
	}

	endpoint := parseLBEndpoint(
		"cluster-a",
		&endpointType.LbEndpoint{},
		&corev3.Locality{Region: "cn", Zone: "hz", SubZone: "a"},
		2,
		3,
	)
	if endpoint.Cluster != "cluster-a" || endpoint.Weight != 3 || endpoint.Priority != 2 {
		t.Fatalf("parseLBEndpoint() = %#v", endpoint)
	}
	if endpoint.Metadata["region"] != "cn" || endpoint.Metadata["health"] != "HEALTHY" {
		t.Fatalf("parseLBEndpoint metadata = %#v", endpoint.Metadata)
	}
}

func TestRouteMatchAndActionParsingEdges(t *testing.T) {
	if got := parseRouteMatch(nil); got != nil {
		t.Fatalf("parseRouteMatch(nil) = %#v, want nil", got)
	}

	parsed := parseRouteMatch(&routeType.RouteMatch{
		PathSpecifier: &routeType.RouteMatch_Path{Path: "/exact"},
		Headers: []*routeType.HeaderMatcher{
			{
				Name:                 "x-exact",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_ExactMatch{ExactMatch: "prod"},
			},
			{
				Name:                 "x-present",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_PresentMatch{PresentMatch: true},
			},
			{
				Name:                 "x-prefix",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_PrefixMatch{PrefixMatch: "pre"},
			},
			{
				Name:                 "x-suffix",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_SuffixMatch{SuffixMatch: "suf"},
			},
			{
				Name: "x-regex",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_SafeRegexMatch{
					SafeRegexMatch: &matcherType.RegexMatcher{Regex: "^user-"},
				},
			},
		},
	})
	if parsed.Path != "/exact" || len(parsed.Headers) != 5 {
		t.Fatalf("parseRouteMatch() = %#v", parsed)
	}
	if parsed.Headers[1].Present != true || parsed.Headers[4].RegexMatch == nil {
		t.Fatalf("parsed headers = %#v", parsed.Headers)
	}

	invalidRegex := parseRouteMatch(&routeType.RouteMatch{
		PathSpecifier: &routeType.RouteMatch_SafeRegex{
			SafeRegex: &matcherType.RegexMatcher{Regex: "["},
		},
	})
	if invalidRegex.Regex != nil {
		t.Fatalf("invalid regex should not be compiled: %#v", invalidRegex.Regex)
	}

	if got := parseRouteAction(nil); got != nil {
		t.Fatalf("parseRouteAction(nil) = %#v, want nil", got)
	}

	single := parseRouteAction(&routeType.RouteAction{
		ClusterSpecifier: &routeType.RouteAction_Cluster{Cluster: "cluster-a"},
	})
	if single.Cluster != "cluster-a" {
		t.Fatalf("parseRouteAction(cluster) = %#v", single)
	}

	weighted := parseRouteAction(&routeType.RouteAction{
		ClusterSpecifier: &routeType.RouteAction_WeightedClusters{
			WeightedClusters: &routeType.WeightedCluster{
				Clusters: []*routeType.WeightedCluster_ClusterWeight{{Name: "canary"}},
			},
		},
	})
	if weighted.WeightedClusters == nil || weighted.WeightedClusters.Clusters[0].Weight != 0 {
		t.Fatalf("parseRouteAction(weighted) = %#v", weighted)
	}
	if weighted.WeightedClusters.TotalWeight != 0 {
		t.Fatalf("TotalWeight = %d, want 0", weighted.WeightedClusters.TotalWeight)
	}
}

func TestEndpointHealthStatusMapping(t *testing.T) {
	tests := map[corev3.HealthStatus]string{
		0: "HEALTHY",
		1: "UNHEALTHY",
		2: "DRAINING",
		3: "TIMEOUT",
		4: "DEGRADED",
		5: "UNKNOWN",
	}

	for status, want := range tests {
		if got := endpointHealthStatus(status); got != want {
			t.Fatalf("endpointHealthStatus(%v) = %q, want %q", status, got, want)
		}
	}

	metadata := parseEndpointMetadata(&corev3.Locality{Region: "cn", Zone: "hz", SubZone: "b"}, 4)
	if metadata["region"] != "cn" || metadata["zone"] != "hz" || metadata["sub_zone"] != "b" {
		t.Fatalf("parseEndpointMetadata(locality) = %#v", metadata)
	}
	if metadata["health"] != "DEGRADED" {
		t.Fatalf("parseEndpointMetadata(health) = %#v", metadata)
	}

	limiter := parseRateLimiter(&corev3.Metadata{
		FilterMetadata: map[string]*structpb.Struct{
			rateLimitMetadataKey: {
				Fields: map[string]*structpb.Value{
					"max_tokens":      structpb.NewNumberValue(10),
					"tokens_per_fill": structpb.NewNumberValue(2),
					"fill_interval":   structpb.NewNumberValue(0.25),
				},
			},
		},
	})
	if limiter.MaxTokens != 10 || limiter.TokensPerFill != 2 ||
		limiter.FillInterval != 250*time.Millisecond {
		t.Fatalf("parseRateLimiter() = %#v", limiter)
	}
}
