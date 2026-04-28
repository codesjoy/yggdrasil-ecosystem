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
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestDecodeDiscoveryResponse(t *testing.T) {
	managerAny, err := anypb.New(&hcmType.HttpConnectionManager{
		RouteSpecifier: &hcmType.HttpConnectionManager_Rds{
			Rds: &hcmType.Rds{RouteConfigName: "route-a"},
		},
	})
	if err != nil {
		t.Fatalf("anypb.New() error = %v", err)
	}

	listenerAny, err := anypb.New(&listenerType.Listener{
		Name: "listener-a",
		FilterChains: []*listenerType.FilterChain{{
			Filters: []*listenerType.Filter{{
				Name:       httpConnectionManagerFilter,
				ConfigType: &listenerType.Filter_TypedConfig{TypedConfig: managerAny},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("anypb.New() error = %v", err)
	}

	events, err := DecodeDiscoveryResponse(typeURLListener, []*anypb.Any{listenerAny})
	if err != nil {
		t.Fatalf("DecodeDiscoveryResponse() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("DecodeDiscoveryResponse() len = %d, want 1", len(events))
	}
	listenerSnapshot := events[0].Data.(*ListenerSnapshot)
	if listenerSnapshot.Route != "route-a" {
		t.Fatalf("listener route = %q, want route-a", listenerSnapshot.Route)
	}

	clusterAny, err := anypb.New(&clusterType.Cluster{
		Name:     "cluster-a",
		LbPolicy: clusterType.Cluster_LEAST_REQUEST,
		OutlierDetection: &clusterType.OutlierDetection{
			Consecutive_5Xx:                wrapperspb.UInt32(7),
			ConsecutiveGatewayFailure:      wrapperspb.UInt32(3),
			ConsecutiveLocalOriginFailure:  wrapperspb.UInt32(2),
			Interval:                       durationpb.New(5 * time.Second),
			BaseEjectionTime:               durationpb.New(30 * time.Second),
			MaxEjectionTime:                durationpb.New(60 * time.Second),
			MaxEjectionPercent:             wrapperspb.UInt32(25),
			EnforcingConsecutive_5Xx:       wrapperspb.UInt32(100),
			EnforcingSuccessRate:           wrapperspb.UInt32(80),
			SuccessRateMinimumHosts:        wrapperspb.UInt32(5),
			SuccessRateRequestVolume:       wrapperspb.UInt32(50),
			SuccessRateStdevFactor:         wrapperspb.UInt32(1200),
			FailurePercentageThreshold:     wrapperspb.UInt32(70),
			EnforcingFailurePercentage:     wrapperspb.UInt32(100),
			FailurePercentageMinimumHosts:  wrapperspb.UInt32(4),
			FailurePercentageRequestVolume: wrapperspb.UInt32(40),
		},
		Metadata: &corev3.Metadata{
			FilterMetadata: map[string]*structpb.Struct{
				rateLimitMetadataKey: {
					Fields: map[string]*structpb.Value{
						"max_tokens":      structpb.NewNumberValue(10),
						"tokens_per_fill": structpb.NewNumberValue(2),
						"fill_interval":   structpb.NewNumberValue(0.5),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("anypb.New() error = %v", err)
	}

	events, err = DecodeDiscoveryResponse(typeURLCluster, []*anypb.Any{clusterAny})
	if err != nil {
		t.Fatalf("DecodeDiscoveryResponse() error = %v", err)
	}
	clusterSnapshot := events[0].Data.(*ClusterSnapshot)
	if clusterSnapshot.Policy.LBPolicy != "least_request" {
		t.Fatalf("LBPolicy = %q, want least_request", clusterSnapshot.Policy.LBPolicy)
	}
	if clusterSnapshot.Policy.OutlierDetection == nil ||
		clusterSnapshot.Policy.OutlierDetection.Consecutive5xx != 7 {
		t.Fatalf("outlier detection not parsed: %#v", clusterSnapshot.Policy.OutlierDetection)
	}
	if clusterSnapshot.Policy.RateLimiter == nil ||
		clusterSnapshot.Policy.RateLimiter.FillInterval != 500*time.Millisecond {
		t.Fatalf("rate limiter not parsed: %#v", clusterSnapshot.Policy.RateLimiter)
	}

	endpointAny, err := anypb.New(&endpointType.ClusterLoadAssignment{
		ClusterName: "cluster-a",
		Endpoints: []*endpointType.LocalityLbEndpoints{{
			LoadBalancingWeight: wrapperspb.UInt32(2),
			Priority:            1,
			LbEndpoints: []*endpointType.LbEndpoint{{
				HostIdentifier: &endpointType.LbEndpoint_Endpoint{
					Endpoint: &endpointType.Endpoint{
						Address: &corev3.Address{
							Address: &corev3.Address_SocketAddress{
								SocketAddress: &corev3.SocketAddress{
									Address: "127.0.0.1",
									PortSpecifier: &corev3.SocketAddress_PortValue{
										PortValue: 8080,
									},
								},
							},
						},
					},
				},
				LoadBalancingWeight: wrapperspb.UInt32(5),
			}},
		}},
	})
	if err != nil {
		t.Fatalf("anypb.New() error = %v", err)
	}

	events, err = DecodeDiscoveryResponse(typeURLEndpoint, []*anypb.Any{endpointAny})
	if err != nil {
		t.Fatalf("DecodeDiscoveryResponse() error = %v", err)
	}
	endpointSnapshot := events[0].Data.(*EDSSnapshot)
	if len(endpointSnapshot.Endpoints) != 1 {
		t.Fatalf("endpoints len = %d, want 1", len(endpointSnapshot.Endpoints))
	}
	if endpointSnapshot.Endpoints[0].Cluster != "cluster-a" {
		t.Fatalf("endpoint cluster = %q, want cluster-a", endpointSnapshot.Endpoints[0].Cluster)
	}
	if endpointSnapshot.Endpoints[0].Weight != 10 {
		t.Fatalf("endpoint weight = %d, want 10", endpointSnapshot.Endpoints[0].Weight)
	}
}

func TestDecodeDiscoveryResponseUnknownType(t *testing.T) {
	if _, err := DecodeDiscoveryResponse("unknown/type", []*anypb.Any{{}}); err == nil {
		t.Fatal("DecodeDiscoveryResponse() expected error for unknown type")
	}
}

func TestParseRouteWeightedClusters(t *testing.T) {
	events := parseRoute(&routeType.RouteConfiguration{
		Name: "route-a",
		VirtualHosts: []*routeType.VirtualHost{{
			Name:    "default",
			Domains: []string{"*"},
			Routes: []*routeType.Route{{
				Match: &routeType.RouteMatch{
					PathSpecifier: &routeType.RouteMatch_Prefix{Prefix: "/"},
				},
				Action: &routeType.Route_Route{
					Route: &routeType.RouteAction{
						ClusterSpecifier: &routeType.RouteAction_WeightedClusters{
							WeightedClusters: &routeType.WeightedCluster{
								Clusters: []*routeType.WeightedCluster_ClusterWeight{
									{Name: "stable", Weight: wrapperspb.UInt32(80)},
									{Name: "canary", Weight: wrapperspb.UInt32(20)},
								},
								TotalWeight: wrapperspb.UInt32(100), //nolint:staticcheck
							},
						},
					},
				},
			}},
		}},
	})

	if len(events) != 1 {
		t.Fatalf("parseRoute() len = %d, want 1", len(events))
	}
	routeSnapshot := events[0].Data.(*RouteSnapshot)
	action := routeSnapshot.Vhosts[0].Routes[0].Action
	if action == nil || action.WeightedClusters == nil {
		t.Fatalf("weighted clusters not parsed: %#v", action)
	}
	if len(action.WeightedClusters.Clusters) != 2 {
		t.Fatalf("clusters len = %d, want 2", len(action.WeightedClusters.Clusters))
	}
}
