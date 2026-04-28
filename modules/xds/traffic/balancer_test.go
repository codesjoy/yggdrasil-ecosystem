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

package traffic

import (
	"context"
	"sync/atomic"
	"testing"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

type mockClient struct {
	address string
	port    int
}

func (m *mockClient) NewStream(
	ctx context.Context,
	desc *stream.Desc,
	method string,
) (stream.ClientStream, error) {
	return nil, nil
}

func (m *mockClient) Close() error {
	return nil
}

func (m *mockClient) Protocol() string {
	return "mock"
}

func (m *mockClient) State() remote.State {
	return remote.Ready
}

func (m *mockClient) Connect() {
}

func (m *mockClient) Address() string {
	return m.address
}

func (m *mockClient) Port() int {
	return m.port
}

type mockBalancerClient struct {
	state balancer.State
}

func (m *mockBalancerClient) Update(state balancer.State) {
	m.state = state
}

func (m *mockBalancerClient) Resolve() balancer.State {
	return m.state
}

func (m *mockBalancerClient) UpdateState(state balancer.State) {
	m.state = state
}

func (m *mockBalancerClient) NewRemoteClient(
	endpoint resolver.Endpoint,
	opts balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	return &mockClient{}, nil
}

func TestNewBalancer(t *testing.T) {
	cli := &mockBalancerClient{}
	b, err := newXdsBalancer("test", "", cli)
	if err != nil {
		t.Fatalf("newXdsBalancer() error = %v", err)
	}

	if b == nil {
		t.Fatal("newXdsBalancer() returned nil")
	}

	if b.Type() != "xds" {
		t.Errorf("Type() = %v, want xds", b.Type())
	}
}

func TestXdsBalancer_UpdateState(t *testing.T) {
	cli := &mockBalancerClient{}
	b, _ := newXdsBalancer("test", "", cli)

	endpoints := []resolver.Endpoint{
		resolver.BaseEndpoint{
			Address:  "127.0.0.1:8080",
			Protocol: "grpc",
			Attributes: map[string]any{
				xdsresource.AttributeEndpointCluster:  "test-cluster",
				xdsresource.AttributeEndpointWeight:   uint32(10),
				xdsresource.AttributeEndpointPriority: uint32(0),
			},
		},
	}

	state := resolver.BaseState{
		Endpoints: endpoints,
		Attributes: map[string]any{
			xdsresource.AttributeRoutes: []*xdsresource.VirtualHost{
				{
					Name:    "test-route",
					Domains: []string{"*"},
					Routes: []*xdsresource.Route{
						{
							Match: &xdsresource.RouteMatch{Prefix: ""},
							Action: &xdsresource.RouteAction{
								Cluster: "test-cluster",
							},
						},
					},
				},
			},
			xdsresource.AttributeClusters: map[string]clusterPolicy{
				"test-cluster": {LBPolicy: "round_robin"},
			},
		},
	}

	b.UpdateState(state)
}

func TestXdsBalancer_Pick(t *testing.T) {
	cli := &mockBalancerClient{}
	b, _ := newXdsBalancer("test", "", cli)

	endpoints := []resolver.Endpoint{
		resolver.BaseEndpoint{
			Address:  "127.0.0.1:8080",
			Protocol: "grpc",
			Attributes: map[string]any{
				xdsresource.AttributeEndpointCluster:  "test-cluster",
				xdsresource.AttributeEndpointWeight:   uint32(10),
				xdsresource.AttributeEndpointPriority: uint32(0),
			},
		},
	}

	state := resolver.BaseState{
		Endpoints: endpoints,
		Attributes: map[string]any{
			xdsresource.AttributeRoutes: []*xdsresource.VirtualHost{
				{
					Name:    "test-route",
					Domains: []string{"*"},
					Routes: []*xdsresource.Route{
						{
							Match: &xdsresource.RouteMatch{Prefix: ""},
							Action: &xdsresource.RouteAction{
								Cluster: "test-cluster",
							},
						},
					},
				},
			},
			xdsresource.AttributeClusters: map[string]clusterPolicy{
				"test-cluster": {LBPolicy: "round_robin"},
			},
		},
	}

	b.UpdateState(state)

	pickerState := cli.Resolve()
	if pickerState.Picker == nil {
		t.Fatal("UpdateState() did not set picker")
	}

	if _, err := pickerState.Picker.Next(balancer.RPCInfo{
		Ctx:    context.Background(),
		Method: "/codesjoy.yggdrasil.example.proto.helloword.GreeterService/SayHello",
	}); err != nil {
		t.Fatalf("picker.Next() error = %v, want nil", err)
	}
}

func TestSelectRoundRobin(t *testing.T) {
	cli := &mockBalancerClient{}
	b, _ := newXdsBalancer("test", "", cli)

	if xb, ok := b.(*xdsBalancer); ok {
		endpoints := []*weightedEndpoint{
			{
				Cluster:  "test",
				Endpoint: xdsresource.Endpoint{Address: "127.0.0.1", Port: 8080},
				Weight:   10,
			},
			{
				Cluster:  "test",
				Endpoint: xdsresource.Endpoint{Address: "127.0.0.2", Port: 8081},
				Weight:   20,
			},
		}

		ep := xb.selectRoundRobin(endpoints)
		if ep == nil {
			t.Fatal("selectRoundRobin() returned nil")
		}
	}
}

func TestSelectRandom(t *testing.T) {
	cli := &mockBalancerClient{}
	b, _ := newXdsBalancer("test", "", cli)

	if xb, ok := b.(*xdsBalancer); ok {
		endpoints := []*weightedEndpoint{
			{Cluster: "test", Endpoint: xdsresource.Endpoint{Address: "127.0.0.1", Port: 8080}},
			{Cluster: "test", Endpoint: xdsresource.Endpoint{Address: "127.0.0.2", Port: 8081}},
		}

		ep := xb.selectRandom(endpoints)
		if ep == nil {
			t.Fatal("selectRandom() returned nil")
		}
	}
}

func TestSelectLeastRequest(t *testing.T) {
	cli := &mockBalancerClient{}
	b, _ := newXdsBalancer("test", "", cli)

	if xb, ok := b.(*xdsBalancer); ok {
		endpoints := []*weightedEndpoint{
			{Cluster: "default", Endpoint: xdsresource.Endpoint{Address: "127.0.0.1", Port: 8080}},
			{Cluster: "default", Endpoint: xdsresource.Endpoint{Address: "127.0.0.2", Port: 8081}},
		}

		ep := xb.selectLeastRequest(endpoints)
		if ep == nil {
			t.Fatal("selectLeastRequest() returned nil")
		}

		key := "127.0.0.1:8080"
		if val := xb.inFlight[key]; val != nil && *val != 1 {
			t.Errorf("selectLeastRequest() did not increment in-flight count")
		}
	}
}

func TestLeastRequest_Report_Bug(t *testing.T) {
	cli := &mockBalancerClient{}
	b, _ := newXdsBalancer("test", "", cli)

	xb, ok := b.(*xdsBalancer)
	if !ok {
		t.Fatal("not xdsBalancer")
	}

	endpoints := []resolver.Endpoint{
		resolver.BaseEndpoint{
			Address: "127.0.0.1:8080",
			Attributes: map[string]any{
				xdsresource.AttributeEndpointCluster: "default",
				xdsresource.AttributeEndpointWeight:  uint32(1),
			},
		},
		resolver.BaseEndpoint{
			Address: "127.0.0.2:8080",
			Attributes: map[string]any{
				xdsresource.AttributeEndpointCluster: "default",
				xdsresource.AttributeEndpointWeight:  uint32(1),
			},
		},
	}

	state := resolver.BaseState{
		Endpoints: endpoints,
		Attributes: map[string]any{
			xdsresource.AttributeRoutes: []*xdsresource.VirtualHost{
				{
					Name:    "default-route",
					Domains: []string{"*"},
					Routes: []*xdsresource.Route{
						{
							Match: &xdsresource.RouteMatch{Prefix: ""}, // Match everything
							Action: &xdsresource.RouteAction{
								Cluster: "default",
							},
						},
					},
				},
			},
			xdsresource.AttributeClusters: map[string]clusterPolicy{
				"default": {LBPolicy: "least_request"},
			},
		},
	}
	xb.UpdateState(state)

	// Simulate 2 requests, one to each endpoint
	// Note: We need to manually manipulate inFlight or mock the picker execution flow to ensure both get picked.
	// Since selectLeastRequest selects based on min in-flight, we can "force" the state.

	// 1. Pick first endpoint
	picker := xb.buildPicker()
	pr1, err := picker.Next(balancer.RPCInfo{Ctx: context.Background()})
	if err != nil {
		t.Fatalf("first pick failed: %v", err)
	}

	// 2. Pick second endpoint
	_, err = picker.Next(balancer.RPCInfo{Ctx: context.Background()})
	if err != nil {
		t.Fatalf("second pick failed: %v", err)
	}

	// Verify both are in-flight
	xb.mu.Lock()
	count1 := atomic.LoadInt32(xb.inFlight["127.0.0.1:8080"])
	count2 := atomic.LoadInt32(xb.inFlight["127.0.0.2:8080"])
	xb.mu.Unlock()

	if count1 != 1 || count2 != 1 {
		t.Fatalf("setup failed: expected both to have 1 in-flight, got %d and %d", count1, count2)
	}

	// 3. Report completion for first endpoint only
	if pr1.(interface{ Report(error) }) != nil {
		pr1.(interface{ Report(error) }).Report(nil)
	}

	// 4. Verify only first endpoint was decremented
	xb.mu.Lock()
	count1After := atomic.LoadInt32(xb.inFlight["127.0.0.1:8080"])
	count2After := atomic.LoadInt32(xb.inFlight["127.0.0.2:8080"])
	xb.mu.Unlock()

	if count1After != 0 {
		t.Errorf("expected count1 to be 0, got %d", count1After)
	}
	if count2After != 1 {
		t.Errorf("BUG DETECTED: expected count2 to remain 1, got %d", count2After)
	}
}
