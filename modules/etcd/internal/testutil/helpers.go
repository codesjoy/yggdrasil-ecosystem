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

package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// FakeClient is a configurable etcd client double used by unit tests.
type FakeClient struct {
	GetFunc       func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	PutFunc       func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error)
	DeleteFunc    func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
	GrantFunc     func(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	KeepAliveFunc func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	WatchFunc     func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan
	CloseFunc     func() error
}

var _ internalclient.Client = (*FakeClient)(nil)

// Get implements the internal etcd client interface.
func (f *FakeClient) Get(
	ctx context.Context,
	key string,
	opts ...clientv3.OpOption,
) (*clientv3.GetResponse, error) {
	if f.GetFunc != nil {
		return f.GetFunc(ctx, key, opts...)
	}
	return &clientv3.GetResponse{}, nil
}

// Put implements the internal etcd client interface.
func (f *FakeClient) Put(
	ctx context.Context,
	key string,
	val string,
	opts ...clientv3.OpOption,
) (*clientv3.PutResponse, error) {
	if f.PutFunc != nil {
		return f.PutFunc(ctx, key, val, opts...)
	}
	return &clientv3.PutResponse{}, nil
}

// Delete implements the internal etcd client interface.
func (f *FakeClient) Delete(
	ctx context.Context,
	key string,
	opts ...clientv3.OpOption,
) (*clientv3.DeleteResponse, error) {
	if f.DeleteFunc != nil {
		return f.DeleteFunc(ctx, key, opts...)
	}
	return &clientv3.DeleteResponse{}, nil
}

// Grant implements the internal etcd client interface.
func (f *FakeClient) Grant(
	ctx context.Context,
	ttl int64,
) (*clientv3.LeaseGrantResponse, error) {
	if f.GrantFunc != nil {
		return f.GrantFunc(ctx, ttl)
	}
	return &clientv3.LeaseGrantResponse{}, nil
}

// KeepAlive implements the internal etcd client interface.
func (f *FakeClient) KeepAlive(
	ctx context.Context,
	id clientv3.LeaseID,
) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if f.KeepAliveFunc != nil {
		return f.KeepAliveFunc(ctx, id)
	}
	ch := make(chan *clientv3.LeaseKeepAliveResponse)
	close(ch)
	return ch, nil
}

// Watch implements the internal etcd client interface.
func (f *FakeClient) Watch(
	ctx context.Context,
	key string,
	opts ...clientv3.OpOption,
) clientv3.WatchChan {
	if f.WatchFunc != nil {
		return f.WatchFunc(ctx, key, opts...)
	}
	ch := make(chan clientv3.WatchResponse)
	close(ch)
	return ch
}

// Close implements the internal etcd client interface.
func (f *FakeClient) Close() error {
	if f.CloseFunc != nil {
		return f.CloseFunc()
	}
	return nil
}

// DemoInstance is a simple registry instance used by tests.
type DemoInstance struct {
	NamespaceValue string
	NameValue      string
	VersionValue   string
	RegionValue    string
	ZoneValue      string
	CampusValue    string
	MetadataValue  map[string]string
	EndpointsValue []yregistry.Endpoint
}

// Region returns the instance region.
func (d DemoInstance) Region() string { return d.RegionValue }

// Zone returns the instance zone.
func (d DemoInstance) Zone() string { return d.ZoneValue }

// Campus returns the instance campus.
func (d DemoInstance) Campus() string { return d.CampusValue }

// Namespace returns the instance namespace.
func (d DemoInstance) Namespace() string { return d.NamespaceValue }

// Name returns the instance name.
func (d DemoInstance) Name() string { return d.NameValue }

// Version returns the instance version.
func (d DemoInstance) Version() string { return d.VersionValue }

// Metadata returns the instance metadata.
func (d DemoInstance) Metadata() map[string]string { return d.MetadataValue }

// Endpoints returns the instance endpoints.
func (d DemoInstance) Endpoints() []yregistry.Endpoint { return d.EndpointsValue }

// DemoEndpoint is a simple registry endpoint used by tests.
type DemoEndpoint struct {
	SchemeValue   string
	AddressValue  string
	MetadataValue map[string]string
}

// Scheme returns the endpoint scheme.
func (d DemoEndpoint) Scheme() string { return d.SchemeValue }

// Address returns the endpoint address.
func (d DemoEndpoint) Address() string { return d.AddressValue }

// Metadata returns the endpoint metadata.
func (d DemoEndpoint) Metadata() map[string]string { return d.MetadataValue }

// CaptureWatcher records resolver states sent by the resolver.
type CaptureWatcher struct {
	mu     sync.Mutex
	states []yresolver.State
	ch     chan yresolver.State
}

// NewCaptureWatcher creates a buffered resolver watcher.
func NewCaptureWatcher(buffer int) *CaptureWatcher {
	return &CaptureWatcher{ch: make(chan yresolver.State, buffer)}
}

// UpdateState records one resolver state.
func (w *CaptureWatcher) UpdateState(state yresolver.State) {
	w.mu.Lock()
	w.states = append(w.states, state)
	w.mu.Unlock()
	select {
	case w.ch <- state:
	default:
	}
}

// Channel returns the buffered state channel.
func (w *CaptureWatcher) Channel() <-chan yresolver.State {
	return w.ch
}

// GetResp builds one etcd range response with the supplied revision and keys.
func GetResp(revision int64, kvs ...*mvccpb.KeyValue) *clientv3.GetResponse {
	return &clientv3.GetResponse{
		Header: &pb.ResponseHeader{Revision: revision},
		Kvs:    kvs,
	}
}

// KV builds one etcd key-value pair.
func KV(key string, value string) *mvccpb.KeyValue {
	return &mvccpb.KeyValue{Key: []byte(key), Value: []byte(value)}
}

// ImmediateAfter returns a timer channel that fires immediately.
func ImmediateAfter(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

// MustReceiveState waits for one resolver state and fails the test on timeout.
func MustReceiveState(t testing.TB, ch <-chan yresolver.State) yresolver.State {
	t.Helper()
	select {
	case state := <-ch:
		return state
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for state")
		return nil
	}
}

// MustNotReceiveState verifies that no resolver state arrives within a short window.
func MustNotReceiveState(t testing.TB, ch <-chan yresolver.State) {
	t.Helper()
	select {
	case state := <-ch:
		t.Fatalf("unexpected state: %#v", state)
	case <-time.After(150 * time.Millisecond):
	}
}

// BoolPtr returns a pointer to one bool value.
func BoolPtr(value bool) *bool {
	return &value
}

// UseClientConfigs installs one named client config loader for the duration of the test.
func UseClientConfigs(t testing.TB, configs map[string]internalclient.Config) {
	t.Helper()
	cloned := make(map[string]internalclient.Config, len(configs))
	for name, cfg := range configs {
		cloned[name] = cfg
	}
	internalclient.ConfigureConfigLoader(func(name string) internalclient.Config {
		return cloned[internalclient.ResolveName(name)]
	})
	t.Cleanup(func() { internalclient.ConfigureConfigLoader(nil) })
}
