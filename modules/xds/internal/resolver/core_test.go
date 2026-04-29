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
	"errors"
	"reflect"
	"testing"
	"time"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

type fakeADS struct {
	started bool
	closed  bool
	lds     []string
	rds     []string
	cds     []string
	eds     []string
	err     error
}

func (f *fakeADS) Start() error {
	f.started = true
	return f.err
}

func (f *fakeADS) UpdateSubscriptions(lds, rds, cds, eds []string) {
	f.lds = append([]string(nil), lds...)
	f.rds = append([]string(nil), rds...)
	f.cds = append([]string(nil), cds...)
	f.eds = append([]string(nil), eds...)
}

func (f *fakeADS) Close() {
	f.closed = true
}

type stateRecorder struct {
	ch chan yresolver.State
}

func (r *stateRecorder) UpdateState(state yresolver.State) {
	select {
	case r.ch <- state:
	default:
	}
}

func TestResolverCoreSubscriptionsAndNotifications(t *testing.T) {
	oldFactory := adsClientFactory
	fake := &fakeADS{}
	adsClientFactory = func(
		Config,
		func(xdsresource.DiscoveryEvent),
	) (adsSubscriptionClient, error) {
		return fake, nil
	}
	t.Cleanup(func() { adsClientFactory = oldFactory })

	resolverAny, err := NewResolver("default", Config{
		Protocol:   "grpc",
		ServiceMap: map[string]string{"svc": "listener-1"},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	instance := resolverAny.(*xdsResolver)
	recorder := &stateRecorder{ch: make(chan yresolver.State, 8)}

	if err := instance.AddWatch("svc", recorder); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}
	if !fake.started {
		t.Fatal("ADS client was not started")
	}
	if len(fake.lds) != 1 || fake.lds[0] != "listener-1" {
		t.Fatalf("unexpected LDS subscriptions: %#v", fake.lds)
	}

	instance.core.handleDiscoveryEvent(xdsresource.DiscoveryEvent{
		Typ:  xdsresource.ListenerAdded,
		Name: "listener-1",
		Data: &xdsresource.ListenerSnapshot{Route: "route-1"},
	})
	instance.core.handleDiscoveryEvent(xdsresource.DiscoveryEvent{
		Typ:  xdsresource.RouteAdded,
		Name: "route-1",
		Data: &xdsresource.RouteSnapshot{
			Vhosts: []*xdsresource.VirtualHost{{
				Name:    "vh",
				Domains: []string{"*"},
				Routes: []*xdsresource.Route{
					{Action: &xdsresource.RouteAction{Cluster: "cluster-a"}},
					{
						Action: &xdsresource.RouteAction{
							WeightedClusters: &xdsresource.WeightedClusters{
								Clusters: []*xdsresource.WeightedCluster{
									{Name: "cluster-b", Weight: 10},
								},
							},
						},
					},
				},
			}},
		},
	})
	instance.core.handleDiscoveryEvent(xdsresource.DiscoveryEvent{
		Typ:  xdsresource.ClusterAdded,
		Name: "cluster-a",
		Data: &xdsresource.ClusterSnapshot{
			Policy: xdsresource.ClusterPolicy{LBPolicy: "least_request"},
		},
	})
	instance.core.handleDiscoveryEvent(xdsresource.DiscoveryEvent{
		Typ:  xdsresource.EndpointAdded,
		Name: "cluster-a",
		Data: &xdsresource.EDSSnapshot{
			Endpoints: []*xdsresource.WeightedEndpoint{{
				Cluster:  "cluster-a",
				Endpoint: xdsresource.Endpoint{Address: "127.0.0.1", Port: 8080},
				Weight:   5,
				Priority: 1,
				Metadata: map[string]string{"env": "test"},
			}},
		},
	})

	if len(fake.rds) != 1 || fake.rds[0] != "route-1" {
		t.Fatalf("unexpected RDS subscriptions: %#v", fake.rds)
	}
	if len(fake.cds) < 2 || len(fake.eds) < 2 {
		t.Fatalf("unexpected CDS/EDS subscriptions: %#v %#v", fake.cds, fake.eds)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case state := <-recorder.ch:
			if len(state.GetEndpoints()) == 0 {
				continue
			}
			endpoint := state.GetEndpoints()[0]
			if endpoint.GetAddress() != "127.0.0.1:8080" {
				t.Fatalf("endpoint address = %s, want 127.0.0.1:8080", endpoint.GetAddress())
			}
			if endpoint.GetAttributes()[xdsresource.AttributeEndpointCluster] != "cluster-a" {
				t.Fatalf("endpoint cluster attr = %#v", endpoint.GetAttributes())
			}

			clusterPolicies, ok := state.GetAttributes()[xdsresource.AttributeClusters].(map[string]xdsresource.ClusterPolicy)
			if !ok {
				t.Fatalf(
					"cluster policies type = %T",
					state.GetAttributes()[xdsresource.AttributeClusters],
				)
			}
			if clusterPolicies["cluster-a"].LBPolicy != "least_request" {
				t.Fatalf("cluster policy = %#v", clusterPolicies["cluster-a"])
			}
			goto done
		case <-deadline:
			t.Fatal("timeout waiting for resolver state update")
		}
	}

done:
	if err := instance.DelWatch("svc", recorder); err != nil {
		t.Fatalf("DelWatch() error = %v", err)
	}
	if len(instance.core.apps) != 0 {
		t.Fatalf("apps should be empty after DelWatch: %#v", instance.core.apps)
	}
	if !fake.closed {
		t.Fatal("ADS client should be closed when last watch is removed")
	}
}

func TestDecodeConfig(t *testing.T) {
	cfg := DecodeConfig(map[string]any{
		"server": map[string]any{
			"address": "127.0.0.1:19000",
			"timeout": "3s",
		},
		"node": map[string]any{
			"id":       "node-a",
			"cluster":  "cluster-a",
			"metadata": map[string]string{"env": "test"},
			"locality": map[string]any{"region": "cn"},
		},
		"service_map": map[string]string{"svc": "listener-1"},
		"retry": map[string]any{
			"max_retries": 7,
			"backoff":     "250ms",
		},
		"max_retries": 5,
	})
	if cfg.Server.Address != "127.0.0.1:19000" || cfg.Node.ID != "node-a" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.Retry.MaxRetries != 7 || cfg.MaxRetries != 5 {
		t.Fatalf("retry config not decoded: %#v", cfg)
	}
	if cfg.Node.Locality == nil || cfg.Node.Locality.Region != "cn" {
		t.Fatalf("locality not decoded: %#v", cfg.Node.Locality)
	}
}

func TestDecodeConfigDefaultsAndFallbacks(t *testing.T) {
	defaults := DefaultResolverConfig()

	if got := DecodeConfig(nil); !reflect.DeepEqual(got, defaults) {
		t.Fatalf("DecodeConfig(nil) = %#v, want %#v", got, defaults)
	}

	got := DecodeConfig(map[string]any{
		"server": []string{"bad-shape"},
	})
	if !reflect.DeepEqual(got, defaults) {
		t.Fatalf("DecodeConfig(invalid) = %#v, want defaults %#v", got, defaults)
	}
}

func TestResolverProviderAndListenerName(t *testing.T) {
	var loaded string
	provider := Provider(func(name string) Config {
		loaded = name
		return Config{
			ServiceMap: map[string]string{"svc": "listener-a"},
		}
	})

	if provider.Type() != "xds" {
		t.Fatalf("provider.Type() = %q, want xds", provider.Type())
	}

	resolverAny, err := provider.New("svc")
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if loaded != "svc" {
		t.Fatalf("loader called with %q, want svc", loaded)
	}

	instance := resolverAny.(*xdsResolver)
	if instance.Type() != "xds" {
		t.Fatalf("instance.Type() = %q, want xds", instance.Type())
	}
	if got := instance.listenerName("svc"); got != "listener-a" {
		t.Fatalf("listenerName(mapped) = %q, want listener-a", got)
	}
	if got := instance.listenerName("other"); got != "other" {
		t.Fatalf("listenerName(fallback) = %q, want other", got)
	}
}

func TestAddWatchErrorsAndMultipleWatchers(t *testing.T) {
	t.Run("factory error", func(t *testing.T) {
		oldFactory := adsClientFactory
		t.Cleanup(func() { adsClientFactory = oldFactory })

		expectedErr := errors.New("factory error")
		adsClientFactory = func(
			Config,
			func(xdsresource.DiscoveryEvent),
		) (adsSubscriptionClient, error) {
			return nil, expectedErr
		}

		resolverAny, err := NewResolver("default", Config{})
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}

		recorder := &stateRecorder{ch: make(chan yresolver.State, 1)}
		err = resolverAny.AddWatch("svc", recorder)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("AddWatch() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("start error", func(t *testing.T) {
		oldFactory := adsClientFactory
		t.Cleanup(func() { adsClientFactory = oldFactory })

		expectedErr := errors.New("start error")
		adsClientFactory = func(
			Config,
			func(xdsresource.DiscoveryEvent),
		) (adsSubscriptionClient, error) {
			return &fakeADS{err: expectedErr}, nil
		}

		resolverAny, err := NewResolver("default", Config{})
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}

		recorder := &stateRecorder{ch: make(chan yresolver.State, 1)}
		err = resolverAny.AddWatch("svc", recorder)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("AddWatch() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("keep ads while watcher remains", func(t *testing.T) {
		oldFactory := adsClientFactory
		t.Cleanup(func() { adsClientFactory = oldFactory })

		fake := &fakeADS{}
		adsClientFactory = func(
			Config,
			func(xdsresource.DiscoveryEvent),
		) (adsSubscriptionClient, error) {
			return fake, nil
		}

		resolverAny, err := NewResolver("default", Config{})
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}

		instance := resolverAny.(*xdsResolver)
		recorderA := &stateRecorder{ch: make(chan yresolver.State, 1)}
		recorderB := &stateRecorder{ch: make(chan yresolver.State, 1)}

		if err := instance.AddWatch("svc", recorderA); err != nil {
			t.Fatalf("AddWatch(recorderA) error = %v", err)
		}
		if err := instance.AddWatch("svc", recorderB); err != nil {
			t.Fatalf("AddWatch(recorderB) error = %v", err)
		}
		if err := instance.DelWatch("svc", recorderA); err != nil {
			t.Fatalf("DelWatch(recorderA) error = %v", err)
		}

		if fake.closed {
			t.Fatal("ADS client was closed while another watcher still existed")
		}
		if got := len(instance.watchers["svc"]); got != 1 {
			t.Fatalf("watcher count = %d, want 1", got)
		}
	})
}
