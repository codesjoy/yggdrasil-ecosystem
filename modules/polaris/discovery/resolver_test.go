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

package discovery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/internal/sdk"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	polarisgo "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type fakeConsumer struct {
	reqs []*polarisgo.GetInstancesRequest
	resp *model.InstancesResponse
	err  error
}

func (f *fakeConsumer) GetInstances(
	req *polarisgo.GetInstancesRequest,
) (*model.InstancesResponse, error) {
	f.reqs = append(f.reqs, req)
	return f.resp, f.err
}

type fakeCBStatus struct{}

func (s *fakeCBStatus) GetCircuitBreaker() string            { return "" }
func (s *fakeCBStatus) GetStatus() model.Status              { return 0 }
func (s *fakeCBStatus) GetStartTime() time.Time              { return time.Time{} }
func (s *fakeCBStatus) GetFallbackInfo() *model.FallbackInfo { return nil }
func (s *fakeCBStatus) SetFallbackInfo(*model.FallbackInfo)  {}

type fakeInstance struct {
	namespace string
	service   string
	id        string
	host      string
	port      uint32
	protocol  string
	version   string
	weight    int
	priority  uint32
	metadata  map[string]string
	healthy   bool
	isolated  bool
	region    string
	zone      string
	campus    string
	revision  string
	ttl       int64
}

func (i *fakeInstance) GetInstanceKey() model.InstanceKey { return model.InstanceKey{} }
func (i *fakeInstance) GetNamespace() string              { return i.namespace }
func (i *fakeInstance) GetService() string                { return i.service }
func (i *fakeInstance) GetId() string                     { return i.id } //nolint:revive
func (i *fakeInstance) GetHost() string                   { return i.host }
func (i *fakeInstance) GetPort() uint32                   { return i.port }
func (i *fakeInstance) GetVpcId() string                  { return "" } //nolint:revive
func (i *fakeInstance) GetProtocol() string               { return i.protocol }
func (i *fakeInstance) GetVersion() string                { return i.version }
func (i *fakeInstance) GetWeight() int                    { return i.weight }
func (i *fakeInstance) GetPriority() uint32               { return i.priority }
func (i *fakeInstance) GetMetadata() map[string]string    { return i.metadata }
func (i *fakeInstance) GetLogicSet() string               { return "" }
func (i *fakeInstance) GetCircuitBreakerStatus() model.CircuitBreakerStatus {
	return &fakeCBStatus{}
}
func (i *fakeInstance) IsHealthy() bool           { return i.healthy }
func (i *fakeInstance) IsIsolated() bool          { return i.isolated }
func (i *fakeInstance) IsEnableHealthCheck() bool { return false }
func (i *fakeInstance) GetRegion() string         { return i.region }
func (i *fakeInstance) GetZone() string           { return i.zone }
func (i *fakeInstance) GetIDC() string            { return i.campus }
func (i *fakeInstance) GetCampus() string         { return i.campus }
func (i *fakeInstance) GetRevision() string       { return i.revision }
func (i *fakeInstance) GetTtl() int64             { return i.ttl } //nolint:revive
func (i *fakeInstance) SetHealthy(status bool)    { i.healthy = status }
func (i *fakeInstance) DeepClone() model.Instance { return i }

func TestResolverFetchStateFiltersProtocolsAndBuildsAttributes(t *testing.T) {
	fc := &fakeConsumer{
		resp: &model.InstancesResponse{
			Revision: "rev",
			Instances: []model.Instance{
				&fakeInstance{
					namespace: "ns",
					service:   "svc",
					id:        "i1",
					host:      "127.0.0.1",
					port:      8080,
					protocol:  "grpc",
					version:   "v1",
					weight:    100,
					priority:  1,
					metadata:  map[string]string{"k": "v"},
				},
				&fakeInstance{
					namespace: "ns",
					service:   "svc",
					id:        "i2",
					host:      "127.0.0.1",
					port:      8081,
					protocol:  "http",
				},
			},
		},
	}
	r := &Resolver{
		name: "default",
		cfg: ResolverConfig{
			Namespace:       "ns",
			Protocols:       []string{"grpc"},
			SkipRouteFilter: true,
			Metadata:        map[string]string{"k": "v"},
			Timeout:         time.Second,
			RetryCount:      2,
		},
		api: fc,
	}

	state, err := r.fetchState(context.Background(), "svc")
	if err != nil {
		t.Fatalf("fetchState() error = %v", err)
	}
	if got := len(fc.reqs); got != 1 {
		t.Fatalf("GetInstances calls = %d, want 1", got)
	}
	req := fc.reqs[0]
	if req.Service != "svc" || req.Namespace != "ns" || !req.SkipRouteFilter {
		t.Fatalf("unexpected request: %+v", req.GetInstancesRequest)
	}
	if req.Metadata["k"] != "v" {
		t.Fatalf("Metadata = %+v, want k=v", req.Metadata)
	}
	if req.Timeout == nil || *req.Timeout != time.Second {
		t.Fatalf("Timeout = %v, want 1s", req.Timeout)
	}
	if req.RetryCount == nil || *req.RetryCount != 2 {
		t.Fatalf("RetryCount = %v, want 2", req.RetryCount)
	}

	if state.GetAttributes()["revision"] != "rev" ||
		state.GetAttributes()["namespace"] != "ns" {
		t.Fatalf("state attributes = %+v", state.GetAttributes())
	}
	endpoints := state.GetEndpoints()
	if len(endpoints) != 1 {
		t.Fatalf("endpoints len = %d, want 1", len(endpoints))
	}
	ep := endpoints[0]
	if ep.GetProtocol() != "grpc" || ep.GetAddress() != "127.0.0.1:8080" {
		t.Fatalf(
			"endpoint = (%s, %s), want (grpc, 127.0.0.1:8080)",
			ep.GetProtocol(),
			ep.GetAddress(),
		)
	}
	attrs := ep.GetAttributes()
	if attrs["instance_id"] != "i1" || attrs["weight"] != 100 ||
		attrs["priority"] != uint32(1) || attrs["version"] != "v1" || attrs["k"] != "v" {
		t.Fatalf("attrs = %+v, want Polaris instance attributes", attrs)
	}
}

func TestResolverFetchStateFiltersMetadata(t *testing.T) {
	fc := &fakeConsumer{
		resp: &model.InstancesResponse{
			Instances: []model.Instance{
				&fakeInstance{
					id:       "stable",
					host:     "127.0.0.1",
					port:     8080,
					protocol: "grpc",
					metadata: map[string]string{"version": "stable"},
				},
				&fakeInstance{
					id:       "canary",
					host:     "127.0.0.1",
					port:     8081,
					protocol: "grpc",
					metadata: map[string]string{"version": "canary"},
				},
			},
		},
	}
	r := &Resolver{
		name: "default",
		cfg: ResolverConfig{
			Namespace: "default",
			Metadata:  map[string]string{"version": "canary"},
		},
		api: fc,
	}

	state, err := r.fetchState(context.Background(), "svc")
	if err != nil {
		t.Fatalf("fetchState() error = %v", err)
	}
	endpoints := state.GetEndpoints()
	if len(endpoints) != 1 {
		t.Fatalf("endpoints len = %d, want 1", len(endpoints))
	}
	if got := endpoints[0].GetAttributes()["instance_id"]; got != "canary" {
		t.Fatalf("instance_id = %v, want canary", got)
	}
}

type testResolverWatcher struct {
	updates chan yresolver.State
}

func (w *testResolverWatcher) UpdateState(state yresolver.State) {
	w.updates <- state
}

func TestResolverConstructorsAndWatchLifecycle(t *testing.T) {
	restoreDiscoveryGlobals(t)

	fc := &fakeConsumer{
		resp: &model.InstancesResponse{
			Instances: []model.Instance{
				&fakeInstance{
					id:       "ins-1",
					host:     "127.0.0.1",
					port:     8080,
					protocol: "grpc",
				},
			},
		},
	}
	newResolverConsumerAPI = func(name string, cfg ResolverConfig) (sdk.ConsumerAPI, error) {
		if name != "svc" {
			t.Fatalf("resolver name = %q", name)
		}
		if len(cfg.Addresses) != 1 || cfg.Addresses[0] != "127.0.0.1:8091" {
			t.Fatalf("resolver cfg addresses = %#v", cfg.Addresses)
		}
		return fc, nil
	}

	resolver, err := NewResolver("svc", ResolverConfig{Addresses: []string{"127.0.0.1:8091"}})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	if resolver.Type() != "polaris" {
		t.Fatalf("Type() = %q", resolver.Type())
	}

	provider := ResolverProvider(func(string) ResolverConfig {
		return ResolverConfig{Addresses: []string{"127.0.0.1:8091"}}
	})
	if provider.Type() != "polaris" {
		t.Fatalf("ResolverProvider().Type() = %q", provider.Type())
	}
	viaProvider, err := provider.New("svc")
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if viaProvider.(*Resolver).Type() != "polaris" {
		t.Fatalf("provider.New() Type() = %q", viaProvider.(*Resolver).Type())
	}

	watcher := &testResolverWatcher{updates: make(chan yresolver.State, 2)}
	lifecycle := &Resolver{
		cfg:      ResolverConfig{RefreshInterval: time.Hour},
		api:      fc,
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}
	if err := lifecycle.AddWatch("svc", watcher); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}
	select {
	case state := <-watcher.updates:
		if len(state.GetEndpoints()) != 1 {
			t.Fatalf("state endpoints = %#v", state.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resolver update")
	}
	if got := len(lifecycle.snapshotWatchers("svc")); got != 1 {
		t.Fatalf("snapshotWatchers() len = %d, want 1", got)
	}
	if err := lifecycle.DelWatch("svc", watcher); err != nil {
		t.Fatalf("DelWatch() error = %v", err)
	}
	if got := len(lifecycle.snapshotWatchers("svc")); got != 0 {
		t.Fatalf("snapshotWatchers() len = %d, want 0", got)
	}
	if _, ok := lifecycle.cancels["svc"]; ok {
		t.Fatal("watch cancel should be removed after last watcher deletion")
	}

	withErr := NewResolverWithError("svc", ResolverConfig{}, errors.New("boom"))
	if err := withErr.AddWatch("svc", watcher); err == nil || err.Error() != "boom" {
		t.Fatalf("AddWatch() error = %v", err)
	}
	if err := withErr.DelWatch("svc", watcher); err == nil || err.Error() != "boom" {
		t.Fatalf("DelWatch() error = %v", err)
	}
	if err := resolver.AddWatch("", watcher); err == nil {
		t.Fatal("AddWatch() should fail for empty app name")
	}
}

func TestResolverFetchAndNotifySkipsErrors(t *testing.T) {
	fc := &fakeConsumer{err: errors.New("fetch failed")}
	watcher := &testResolverWatcher{updates: make(chan yresolver.State, 1)}
	r := &Resolver{
		cfg:      ResolverConfig{},
		api:      fc,
		watchers: map[string]map[yresolver.Client]struct{}{"svc": {watcher: {}}},
		cancels:  map[string]context.CancelFunc{},
	}

	r.fetchAndNotify(context.Background(), "svc")

	select {
	case <-watcher.updates:
		t.Fatal("watcher should not be notified when fetchState fails")
	case <-time.After(100 * time.Millisecond):
	}
}
