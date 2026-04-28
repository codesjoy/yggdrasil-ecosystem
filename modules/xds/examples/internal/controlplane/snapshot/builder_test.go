package snapshot

import (
	"testing"

	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

func TestBuildEndpointsUsesWeightsAndPriority(t *testing.T) {
	builder := NewBuilder("1")
	resources := builder.buildEndpoints([]Endpoint{{
		ClusterName: "sample-cluster",
		Priority:    2,
		Locality: &Locality{
			Region:  "cn",
			Zone:    "sh",
			SubZone: "a",
		},
		Endpoints: []EndpointAddress{
			{Address: "127.0.0.1", Port: 56061, Weight: 3},
			{Address: "127.0.0.1", Port: 56062, Weight: 7},
		},
	}})

	if len(resources) != 1 {
		t.Fatalf("resources len = %d, want 1", len(resources))
	}

	assignment, ok := resources[0].(*endpointv3.ClusterLoadAssignment)
	if !ok {
		t.Fatalf("resource type = %T, want *ClusterLoadAssignment", resources[0])
	}
	if len(assignment.Endpoints) != 1 {
		t.Fatalf("locality endpoints len = %d, want 1", len(assignment.Endpoints))
	}

	group := assignment.Endpoints[0]
	if group.Priority != 2 {
		t.Fatalf("priority = %d, want 2", group.Priority)
	}
	if got := group.Locality.GetRegion(); got != "cn" {
		t.Fatalf("region = %q, want cn", got)
	}
	if got := group.LbEndpoints[0].GetLoadBalancingWeight().GetValue(); got != 3 {
		t.Fatalf("first endpoint weight = %d, want 3", got)
	}
	if got := group.LbEndpoints[1].GetLoadBalancingWeight().GetValue(); got != 7 {
		t.Fatalf("second endpoint weight = %d, want 7", got)
	}
}

func TestBuildRoutesUsesWeightedClustersAndHeaderMatch(t *testing.T) {
	builder := NewBuilder("1")
	resources := builder.buildRoutes([]Route{{
		Name: "sample-route",
		VirtualHosts: []VirtualHost{{
			Name:    "sample",
			Domains: []string{"*"},
			Routes: []RouteMatch{{
				Match: RouteMatchCondition{
					Path: &PathMatchCondition{Prefix: "/"},
					Headers: []HeaderMatchCondition{{
						Name:    "x-release",
						Pattern: "exact",
						Value:   "canary",
					}},
				},
				Route: RouteAction{
					WeightedClusters: &WeightedRouteAction{
						Clusters: []WeightedCluster{
							{Name: "stable-cluster", Weight: 95},
							{Name: "canary-cluster", Weight: 5},
						},
					},
				},
			}},
		}},
	}})

	config, ok := resources[0].(*routev3.RouteConfiguration)
	if !ok {
		t.Fatalf("resource type = %T, want *RouteConfiguration", resources[0])
	}

	builtRoute := config.VirtualHosts[0].Routes[0]
	weighted := builtRoute.GetRoute().GetWeightedClusters()
	if weighted == nil {
		t.Fatal("weighted clusters = nil, want populated action")
	}
	if got := weighted.GetTotalWeight().GetValue(); got != 100 {
		t.Fatalf("total weight = %d, want 100", got)
	}
	if len(weighted.Clusters) != 2 {
		t.Fatalf("cluster count = %d, want 2", len(weighted.Clusters))
	}
	if got := weighted.Clusters[0].GetName(); got != "stable-cluster" {
		t.Fatalf("first cluster = %q, want stable-cluster", got)
	}
	if got := weighted.Clusters[1].GetWeight().GetValue(); got != 5 {
		t.Fatalf("second cluster weight = %d, want 5", got)
	}

	headers := builtRoute.GetMatch().GetHeaders()
	if len(headers) != 1 {
		t.Fatalf("headers len = %d, want 1", len(headers))
	}
	if got := headers[0].GetStringMatch().GetExact(); got != "canary" {
		t.Fatalf("header exact match = %q, want canary", got)
	}
}

func TestBuildRoutesUsesSafeRegexForContainsAndSuffix(t *testing.T) {
	builder := NewBuilder("1")
	resources := builder.buildRoutes([]Route{{
		Name: "sample-route",
		VirtualHosts: []VirtualHost{{
			Name:    "sample",
			Domains: []string{"*"},
			Routes: []RouteMatch{
				{
					Match: RouteMatchCondition{
						Path: &PathMatchCondition{Contains: "/canary/"},
					},
					Route: RouteAction{Cluster: "canary-cluster"},
				},
				{
					Match: RouteMatchCondition{
						Path: &PathMatchCondition{Suffix: "/ready"},
					},
					Route: RouteAction{Cluster: "ready-cluster"},
				},
			},
		}},
	}})

	config := resources[0].(*routev3.RouteConfiguration)
	first := config.VirtualHosts[0].Routes[0].GetMatch().GetSafeRegex()
	second := config.VirtualHosts[0].Routes[1].GetMatch().GetSafeRegex()

	if first == nil || first.Regex != ".*/canary/.*" {
		t.Fatalf("contains regex = %#v, want .*/canary/.*", first)
	}
	if second == nil || second.Regex != ".*/ready$" {
		t.Fatalf("suffix regex = %#v, want .*/ready$", second)
	}
}
