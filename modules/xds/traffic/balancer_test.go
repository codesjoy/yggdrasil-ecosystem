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
	"errors"
	"math/rand"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	rpcmetadata "github.com/codesjoy/yggdrasil/v3/rpc/metadata"
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

type recordingRemoteClient struct {
	address      string
	port         int
	protocol     string
	state        remote.State
	closeErr     error
	connectCount int
	closeCount   int
}

func (c *recordingRemoteClient) NewStream(
	context.Context,
	*stream.Desc,
	string,
) (stream.ClientStream, error) {
	return nil, nil
}

func (c *recordingRemoteClient) Close() error {
	c.closeCount++
	return c.closeErr
}

func (c *recordingRemoteClient) Protocol() string {
	if c.protocol == "" {
		return "grpc"
	}
	return c.protocol
}

func (c *recordingRemoteClient) State() remote.State {
	if c.state == 0 {
		return remote.Ready
	}
	return c.state
}

func (c *recordingRemoteClient) Connect() {
	c.connectCount++
}

func (c *recordingRemoteClient) Address() string {
	return c.address
}

func (c *recordingRemoteClient) Port() int {
	return c.port
}

type recordingBalancerClient struct {
	state       balancer.State
	updateCount int
	newErr      map[string]error
	clients     map[string]*recordingRemoteClient
}

func (c *recordingBalancerClient) UpdateState(state balancer.State) {
	c.state = state
	c.updateCount++
}

func (c *recordingBalancerClient) NewRemoteClient(
	endpoint resolver.Endpoint,
	_ balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	key := endpoint.GetAddress()
	if key == "" {
		key = endpoint.Name()
	}
	if err := c.newErr[key]; err != nil {
		return nil, err
	}
	if c.clients == nil {
		c.clients = make(map[string]*recordingRemoteClient)
	}
	if client := c.clients[key]; client != nil {
		return client, nil
	}

	host, port := splitEndpointAddress(endpoint.GetAddress())
	client := &recordingRemoteClient{
		address:  host,
		port:     port,
		protocol: endpoint.GetProtocol(),
		state:    remote.Ready,
	}
	c.clients[key] = client
	return client, nil
}

func newDeterministicBalancer(t *testing.T, cli *recordingBalancerClient) *xdsBalancer {
	t.Helper()

	balancerAny, err := newXdsBalancer("svc", "", cli)
	if err != nil {
		t.Fatalf("newXdsBalancer() error = %v", err)
	}
	instance := balancerAny.(*xdsBalancer)
	//nolint:gosec // Deterministic pseudo-random source is required for test assertions.
	instance.rng = rand.New(rand.NewSource(1))
	return instance
}

func testRoute(cluster string, weighted *xdsresource.WeightedClusters) []*xdsresource.VirtualHost {
	return []*xdsresource.VirtualHost{{
		Name:    "default",
		Domains: []string{"*"},
		Routes: []*xdsresource.Route{{
			Match:  &xdsresource.RouteMatch{Prefix: "/"},
			Action: &xdsresource.RouteAction{Cluster: cluster, WeightedClusters: weighted},
		}},
	}}
}

func testState(
	endpoints []resolver.Endpoint,
	vhosts []*xdsresource.VirtualHost,
	policies map[string]clusterPolicy,
) resolver.State {
	return resolver.BaseState{
		Endpoints: endpoints,
		Attributes: map[string]any{
			xdsresource.AttributeRoutes:   vhosts,
			xdsresource.AttributeClusters: policies,
		},
	}
}

func TestBalancerProviderAndConfig(t *testing.T) {
	provider := BalancerProvider()
	if provider.Type() != "xds" {
		t.Fatalf("provider.Type() = %q, want xds", provider.Type())
	}

	cfg := LoadBalancerConfig("svc")
	if got := (&cfg).String(); got != "{}" {
		t.Fatalf("BalancerConfig.String() = %q, want {}", got)
	}

	instance, err := provider.New("svc", "xds", &recordingBalancerClient{})
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if instance.Type() != "xds" {
		t.Fatalf("instance.Type() = %q, want xds", instance.Type())
	}
}

func TestBalancerLifecycleAndStats(t *testing.T) {
	cli := &recordingBalancerClient{
		newErr: map[string]error{"bad:80": errors.New("new remote client failed")},
	}
	instance := newDeterministicBalancer(t, cli)

	first := resolver.BaseEndpoint{
		Address:  "10.0.0.1:8080",
		Protocol: "grpc",
		Attributes: map[string]any{
			xdsresource.AttributeEndpointCluster:  "cluster-a",
			xdsresource.AttributeEndpointWeight:   uint32(3),
			xdsresource.AttributeEndpointPriority: uint32(0),
		},
	}
	second := resolver.BaseEndpoint{
		Address:  "10.0.0.2:8080",
		Protocol: "grpc",
		Attributes: map[string]any{
			xdsresource.AttributeEndpointCluster:  "cluster-a",
			xdsresource.AttributeEndpointWeight:   uint32(1),
			xdsresource.AttributeEndpointPriority: uint32(1),
		},
	}
	bad := resolver.BaseEndpoint{
		Address:  "bad:80",
		Protocol: "grpc",
		Attributes: map[string]any{
			xdsresource.AttributeEndpointCluster: "cluster-a",
		},
	}

	policies := map[string]clusterPolicy{
		"cluster-a": {
			LBPolicy: "random",
			CircuitBreaker: &CircuitBreakerConfig{
				MaxRequests: 1,
			},
			OutlierDetection: &OutlierDetectionConfig{
				Consecutive5xx:          1,
				Interval:                time.Millisecond,
				BaseEjectionTime:        time.Millisecond,
				MaxEjectionTime:         2 * time.Millisecond,
				MaxEjectionPercent:      100,
				EnforcingConsecutive5xx: 100,
			},
			RateLimiter: &RateLimiterConfig{
				MaxTokens:     1,
				TokensPerFill: 1,
				FillInterval:  time.Millisecond,
			},
		},
	}

	instance.UpdateState(testState(
		[]resolver.Endpoint{first, second, bad},
		testRoute("cluster-a", nil),
		policies,
	))
	defer instance.Close() //nolint:errcheck

	if cli.updateCount != 1 || cli.state.Picker == nil {
		t.Fatalf(
			"unexpected balancer client state: updates=%d picker=%#v",
			cli.updateCount,
			cli.state.Picker,
		)
	}
	if cli.clients["10.0.0.1:8080"].connectCount != 1 {
		t.Fatalf(
			"first client connectCount = %d, want 1",
			cli.clients["10.0.0.1:8080"].connectCount,
		)
	}
	if cli.clients["10.0.0.2:8080"].connectCount != 1 {
		t.Fatalf(
			"second client connectCount = %d, want 1",
			cli.clients["10.0.0.2:8080"].connectCount,
		)
	}
	if _, ok := cli.clients["bad:80"]; ok {
		t.Fatal("unexpected remote client creation for failing endpoint")
	}

	stats := instance.GetStats()
	if len(stats.CircuitBreakers) != 1 || len(stats.OutlierDetectors) != 1 ||
		len(stats.RateLimiters) != 1 {
		t.Fatalf("GetStats() = %#v", stats)
	}

	instance.UpdateRemoteClientState(remote.ClientState{})
	if cli.updateCount != 2 {
		t.Fatalf("UpdateRemoteClientState() updateCount = %d, want 2", cli.updateCount)
	}

	staleClient := cli.clients["10.0.0.1:8080"]
	instance.UpdateState(testState(
		[]resolver.Endpoint{second},
		testRoute("cluster-a", nil),
		policies,
	))
	if staleClient.closeCount != 1 {
		t.Fatalf("stale client closeCount = %d, want 1", staleClient.closeCount)
	}
	if cli.clients["10.0.0.2:8080"].connectCount != 1 {
		t.Fatalf(
			"reused client connectCount = %d, want 1",
			cli.clients["10.0.0.2:8080"].connectCount,
		)
	}

	instance.UpdateState(resolver.BaseState{
		Endpoints: []resolver.Endpoint{second},
		Attributes: map[string]any{
			xdsresource.AttributeRoutes: []*xdsresource.VirtualHost{},
		},
	})
	if len(instance.vhosts) != 0 {
		t.Fatalf("vhosts = %#v, want empty", instance.vhosts)
	}
	if len(instance.clusterPolicies) != 0 ||
		len(instance.circuitBreakers) != 0 ||
		len(instance.outlierDetectors) != 0 ||
		len(instance.rateLimiters) != 0 {
		t.Fatalf("expected policy state reset, got %#v", instance.GetStats())
	}

	cli.clients["10.0.0.2:8080"].closeErr = errors.New("close failed")
	if err := instance.Close(); err == nil || !strings.Contains(err.Error(), "close failed") {
		t.Fatalf("Close() error = %v, want joined close failure", err)
	}
	if instance.remotesClient != nil {
		t.Fatalf("remotesClient = %#v, want nil after Close", instance.remotesClient)
	}
}

func TestPickerBehaviors(t *testing.T) {
	t.Run("request headers", func(t *testing.T) {
		ctx := rpcmetadata.WithOutContext(context.Background(), rpcmetadata.Pairs(
			":path", "/from-metadata",
			"x-env", "prod",
		))
		headers := requestHeaders(ctx)
		if headers[":path"] != "/from-metadata" || headers["x-env"] != "prod" {
			t.Fatalf("requestHeaders() = %#v", headers)
		}
		if empty := requestHeaders(context.Background()); len(empty) != 0 {
			t.Fatalf("requestHeaders(empty) = %#v, want empty", empty)
		}
	})

	t.Run("weighted cluster selection", func(t *testing.T) {
		instance := newDeterministicBalancer(t, &recordingBalancerClient{})
		//nolint:gosec // Deterministic pseudo-random source is required for test assertions.
		expectedRNG := rand.New(rand.NewSource(1))
		randomWeight := expectedRNG.Uint32() % 3
		want := "canary"
		if randomWeight < 2 {
			want = "stable"
		}

		got := instance.selectWeightedCluster(&xdsresource.WeightedClusters{
			Clusters: []*xdsresource.WeightedCluster{
				{Name: "stable", Weight: 2},
				{Name: "canary", Weight: 1},
			},
			TotalWeight: 3,
		})
		if got != want {
			t.Fatalf("selectWeightedCluster() = %q, want %q", got, want)
		}
		if got := instance.selectWeightedCluster(&xdsresource.WeightedClusters{}); got != "" {
			t.Fatalf("selectWeightedCluster(empty) = %q, want empty", got)
		}
		if got := instance.selectWeightedCluster(&xdsresource.WeightedClusters{
			Clusters: []*xdsresource.WeightedCluster{{Name: "stable"}},
		}); got != "stable" {
			t.Fatalf("selectWeightedCluster(zero total) = %q, want stable", got)
		}
	})

	t.Run("no route", func(t *testing.T) {
		instance := newDeterministicBalancer(t, &recordingBalancerClient{})
		picker := instance.buildPicker()
		if _, err := picker.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/Method"}); !errors.Is(
			err,
			balancer.ErrNoAvailableInstance,
		) {
			t.Fatalf("Next() error = %v, want ErrNoAvailableInstance", err)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		instance := newDeterministicBalancer(t, &recordingBalancerClient{
			clients: map[string]*recordingRemoteClient{
				"10.0.0.1:8080": {address: "10.0.0.1", port: 8080, state: remote.Ready},
			},
		})
		limiter := NewRateLimiter(&RateLimiterConfig{
			MaxTokens:     0,
			TokensPerFill: 0,
			FillInterval:  time.Hour,
		})
		defer limiter.Stop()

		instance.vhosts = testRoute("cluster-a", nil)
		instance.endpoints["cluster-a"] = []*weightedEndpoint{{
			Cluster:  "cluster-a",
			Endpoint: xdsresource.Endpoint{Address: "10.0.0.1", Port: 8080},
			Weight:   1,
		}}
		instance.rateLimiters["cluster-a"] = limiter

		_, err := instance.buildPicker().Next(balancer.RPCInfo{
			Ctx:    context.Background(),
			Method: "/svc/Method",
		})
		if !errors.Is(err, errRateLimitExceeded) {
			t.Fatalf("Next() error = %v, want errRateLimitExceeded", err)
		}
	})

	t.Run("circuit breaker and endpoint failures", func(t *testing.T) {
		instance := newDeterministicBalancer(t, &recordingBalancerClient{
			clients: map[string]*recordingRemoteClient{
				"10.0.0.1:8080": {address: "10.0.0.1", port: 8080, state: remote.TransientFailure},
			},
		})
		instance.vhosts = testRoute("cluster-a", nil)
		instance.endpoints["cluster-a"] = []*weightedEndpoint{{
			Cluster:  "cluster-a",
			Endpoint: xdsresource.Endpoint{Address: "10.0.0.1", Port: 8080},
			Weight:   1,
		}}

		openBreaker := NewCircuitBreaker(&CircuitBreakerConfig{MaxRequests: 1})
		if !openBreaker.TryAcquire(ResourceRequest) {
			t.Fatal("pre-acquire request failed")
		}
		instance.circuitBreakers["cluster-a"] = openBreaker
		if _, err := instance.buildPicker().Next(balancer.RPCInfo{
			Ctx:    context.Background(),
			Method: "/svc/Method",
		}); err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
			t.Fatalf("Next() error = %v, want circuit breaker open", err)
		}
		openBreaker.Release(ResourceRequest)

		readyBreaker := NewCircuitBreaker(&CircuitBreakerConfig{MaxRequests: 1})
		instance.circuitBreakers["cluster-a"] = readyBreaker
		if _, err := instance.buildPicker().Next(balancer.RPCInfo{
			Ctx:    context.Background(),
			Method: "/svc/Method",
		}); !errors.Is(err, balancer.ErrNoAvailableInstance) {
			t.Fatalf("Next() error = %v, want ErrNoAvailableInstance", err)
		}
		if stats := readyBreaker.GetStats(); stats.ActiveRequests != 0 {
			t.Fatalf("breaker stats after ready failure = %#v", stats)
		}

		instance.endpoints["cluster-a"] = nil
		if _, err := instance.buildPicker().Next(balancer.RPCInfo{
			Ctx:    context.Background(),
			Method: "/svc/Method",
		}); !errors.Is(err, balancer.ErrNoAvailableInstance) {
			t.Fatalf("Next() error = %v, want ErrNoAvailableInstance", err)
		}
	})

	t.Run("report and remote client", func(t *testing.T) {
		instance := newDeterministicBalancer(t, &recordingBalancerClient{})
		client := &recordingRemoteClient{
			address: "10.0.0.9",
			port:    8080,
			state:   remote.Ready,
		}
		breaker := NewCircuitBreaker(&CircuitBreakerConfig{MaxRequests: 1})
		if !breaker.TryAcquire(ResourceRequest) {
			t.Fatal("breaker acquire failed")
		}
		detector := NewOutlierDetector(&OutlierDetectionConfig{
			Consecutive5xx:          1,
			BaseEjectionTime:        time.Minute,
			MaxEjectionTime:         time.Minute,
			MaxEjectionPercent:      100,
			EnforcingConsecutive5xx: 100,
		})
		key := "10.0.0.9:8080"
		inFlight := int32(1)
		instance.inFlight[key] = &inFlight

		result := &pickResult{
			endpoint:        client,
			balancer:        instance,
			inflightKey:     key,
			circuitBreaker:  breaker,
			outlierDetector: detector,
		}
		if result.RemoteClient() != client {
			t.Fatal("RemoteClient() did not return the selected client")
		}

		result.Report(errors.New("rpc failed"))
		if got := atomic.LoadInt32(instance.inFlight[key]); got != 0 {
			t.Fatalf("inFlight after Report() = %d, want 0", got)
		}
		if stats := breaker.GetStats(); stats.ActiveRequests != 0 {
			t.Fatalf("breaker stats after Report() = %#v", stats)
		}
		if !detector.IsEjected(key) {
			t.Fatal("outlier detector did not eject the failing endpoint")
		}
	})

	t.Run("selection helpers", func(t *testing.T) {
		instance := newDeterministicBalancer(t, &recordingBalancerClient{})
		endpoints := []*weightedEndpoint{
			{
				Cluster:  "cluster-a",
				Endpoint: xdsresource.Endpoint{Address: "10.0.0.1", Port: 8080},
				Weight:   0,
			},
			{
				Cluster:  "cluster-a",
				Endpoint: xdsresource.Endpoint{Address: "10.0.0.2", Port: 8080},
				Weight:   0,
			},
		}
		if got := instance.selectRoundRobin(nil); got != nil {
			t.Fatalf("selectRoundRobin(nil) = %#v, want nil", got)
		}
		if got := instance.selectRoundRobin(endpoints); got != endpoints[0] {
			t.Fatalf("selectRoundRobin(zero weight) = %#v, want first endpoint", got)
		}
		if got := instance.selectRandom(nil); got != nil {
			t.Fatalf("selectRandom(nil) = %#v, want nil", got)
		}
		if got := instance.selectLeastRequest(nil); got != nil {
			t.Fatalf("selectLeastRequest(nil) = %#v, want nil", got)
		}

		endpoints[0].Weight = 1
		endpoints[1].Weight = 2
		instance.inFlight[endpointAddress(endpoints[0])] = new(int32)
		one := int32(1)
		instance.inFlight[endpointAddress(endpoints[1])] = &one
		if got := instance.selectEndpoint("cluster-a", nil); got != nil {
			t.Fatalf("selectEndpoint(missing cluster map) = %#v, want nil", got)
		}

		instance.endpoints["cluster-a"] = endpoints
		instance.clusterPolicies["cluster-a"] = clusterPolicy{LBPolicy: "least_request"}
		if got := instance.selectEndpoint("cluster-a", nil); got != endpoints[0] {
			t.Fatalf("selectEndpoint(least_request) = %#v, want first endpoint", got)
		}

		detector := NewOutlierDetector(&OutlierDetectionConfig{
			Consecutive5xx:          1,
			BaseEjectionTime:        time.Minute,
			MaxEjectionTime:         time.Minute,
			MaxEjectionPercent:      100,
			EnforcingConsecutive5xx: 100,
		})
		detector.ReportResult(endpointAddress(endpoints[0]), errors.New("boom"), 503)
		filtered := filterHealthyEndpoints(endpoints, detector)
		if len(filtered) != 1 || filtered[0] != endpoints[1] {
			t.Fatalf("filterHealthyEndpoints() = %#v", filtered)
		}

		instance.clusterPolicies["cluster-a"] = clusterPolicy{LBPolicy: "random"}
		got := instance.selectEndpoint("cluster-a", detector)
		if got != endpoints[1] {
			t.Fatalf("selectEndpoint(random healthy) = %#v, want second endpoint", got)
		}

		instance.clusterPolicies = map[string]clusterPolicy{}
		got = instance.selectEndpoint("cluster-a", nil)
		if !slices.Contains(endpoints, got) {
			t.Fatalf("selectEndpoint(default round_robin) = %#v, want one of %#v", got, endpoints)
		}
	})
}
