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
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/testutil"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestNewRegistryDefaults(t *testing.T) {
	reg, err := NewRegistry(RegistryConfig{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	defer func() { _ = reg.Close() }()

	if reg.cfg.Prefix != "/yggdrasil/registry" {
		t.Fatalf("prefix = %q, want /yggdrasil/registry", reg.cfg.Prefix)
	}
	if reg.cfg.TTL != 10*time.Second {
		t.Fatalf("ttl = %v, want 10s", reg.cfg.TTL)
	}
	if reg.cfg.RetryInterval != 3*time.Second {
		t.Fatalf("retryInterval = %v, want 3s", reg.cfg.RetryInterval)
	}
}

func TestRegistryBuildKeyValue(t *testing.T) {
	reg := &Registry{cfg: RegistryConfig{Prefix: "/custom/prefix"}}
	inst := testutil.DemoInstance{
		NamespaceValue: "default",
		NameValue:      "svc",
		VersionValue:   "v1",
		RegionValue:    "cn",
		ZoneValue:      "a",
		CampusValue:    "hz",
		MetadataValue:  map[string]string{"env": "dev"},
		EndpointsValue: []yregistry.Endpoint{
			testutil.DemoEndpoint{
				SchemeValue:   "grpc",
				AddressValue:  "127.0.0.1:9000",
				MetadataValue: map[string]string{"tag": "blue"},
			},
		},
	}

	key1, value1, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue() error = %v", err)
	}
	key2, value2, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue() second error = %v", err)
	}
	if key1 != key2 || value1 != value2 {
		t.Fatalf(
			"same instance should produce stable key/value: %q/%q vs %q/%q",
			key1,
			value1,
			key2,
			value2,
		)
	}
	if !strings.HasPrefix(key1, "/custom/prefix/default/svc/") {
		t.Fatalf("key = %q, want custom prefix/namespace/name", key1)
	}

	var rec instanceRecord
	if err := json.Unmarshal([]byte(value1), &rec); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if rec.Name != "svc" || rec.Namespace != "default" || len(rec.Endpoints) != 1 {
		t.Fatalf("record = %#v", rec)
	}

	inst.EndpointsValue = []yregistry.Endpoint{
		testutil.DemoEndpoint{SchemeValue: "grpc", AddressValue: "127.0.0.1:9001"},
	}
	key3, _, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue() changed error = %v", err)
	}
	if key1 == key3 {
		t.Fatalf("different endpoint should produce different key, both = %q", key1)
	}
}

func TestRegistryRegisterDeregisterAndClose(t *testing.T) {
	ctx := context.Background()
	inst := testutil.DemoInstance{
		NamespaceValue: "default",
		NameValue:      "svc",
		VersionValue:   "v1",
		EndpointsValue: []yregistry.Endpoint{
			testutil.DemoEndpoint{SchemeValue: "grpc", AddressValue: "127.0.0.1:9000"},
		},
	}

	var putCount, deleteCount, closeCount int32
	reg := &Registry{
		cfg: RegistryConfig{
			Prefix:        "/yggdrasil/registry",
			KeepAlive:     testutil.BoolPtr(false),
			TTL:           2 * time.Second,
			RetryInterval: time.Millisecond,
		},
		client: &testutil.FakeClient{
			GrantFunc: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
				return &clientv3.LeaseGrantResponse{ID: clientv3.LeaseID(7)}, nil
			},
			PutFunc: func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
				atomic.AddInt32(&putCount, 1)
				return &clientv3.PutResponse{}, nil
			},
			DeleteFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
				atomic.AddInt32(&deleteCount, 1)
				return &clientv3.DeleteResponse{}, nil
			},
			CloseFunc: func() error {
				atomic.AddInt32(&closeCount, 1)
				return nil
			},
		},
		regs:  map[string]registryEntry{},
		close: make(chan struct{}),
		after: testutil.ImmediateAfter,
	}

	if err := reg.Register(ctx, nil); err == nil || !strings.Contains(err.Error(), "nil instance") {
		t.Fatalf("Register(nil) error = %v", err)
	}
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if atomic.LoadInt32(&putCount) != 1 {
		t.Fatalf("putCount = %d, want 1", putCount)
	}
	key, _, _ := reg.buildKeyValue(inst)
	if reg.regs[key].lease != clientv3.LeaseID(7) {
		t.Fatalf("lease = %v, want 7", reg.regs[key].lease)
	}

	if err := reg.Deregister(ctx, nil); err == nil ||
		!strings.Contains(err.Error(), "nil instance") {
		t.Fatalf("Deregister(nil) error = %v", err)
	}
	if err := reg.Deregister(ctx, inst); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if atomic.LoadInt32(&deleteCount) != 1 {
		t.Fatalf("deleteCount = %d, want 1", deleteCount)
	}

	if err := reg.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := reg.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if atomic.LoadInt32(&closeCount) != 1 {
		t.Fatalf("closeCount = %d, want 1", closeCount)
	}
}

func TestRegistryRegisterCancelsExistingEntryAndRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	inst := testutil.DemoInstance{
		NamespaceValue: "default",
		NameValue:      "svc",
		VersionValue:   "v1",
		EndpointsValue: []yregistry.Endpoint{
			testutil.DemoEndpoint{SchemeValue: "grpc", AddressValue: "127.0.0.1:9000"},
		},
	}
	var canceled bool
	reg := &Registry{
		cfg: RegistryConfig{Prefix: "/yggdrasil/registry", KeepAlive: testutil.BoolPtr(false)},
		client: &testutil.FakeClient{
			GrantFunc: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
				return nil, errors.New("grant failed")
			},
		},
		regs:  map[string]registryEntry{},
		close: make(chan struct{}),
		after: testutil.ImmediateAfter,
	}

	builtKey, _, err := (&Registry{cfg: reg.cfg}).buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue() error = %v", err)
	}
	reg.regs[builtKey] = registryEntry{cancel: func() { canceled = true }}
	if err := reg.Register(ctx, inst); err == nil ||
		!strings.Contains(err.Error(), "grant failed") {
		t.Fatalf("Register() error = %v, want grant failed", err)
	}
	if !canceled {
		t.Fatal("existing cancel was not invoked")
	}
	if _, ok := reg.regs[builtKey]; ok {
		t.Fatalf("entry %q should be removed after failed register", builtKey)
	}
}

func TestRegistryKeepAliveLoopBranches(t *testing.T) {
	t.Run("grant error retries then exits", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reg := &Registry{
			cfg: RegistryConfig{TTL: time.Second, RetryInterval: time.Millisecond},
			client: &testutil.FakeClient{
				GrantFunc: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
					cancel()
					return nil, errors.New("grant failed")
				},
			},
			regs:  map[string]registryEntry{},
			close: make(chan struct{}),
			after: testutil.ImmediateAfter,
		}
		reg.keepAliveLoop(ctx, "/key", "value")
	})

	t.Run("keepalive error retries then exits", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reg := &Registry{
			cfg: RegistryConfig{TTL: time.Second, RetryInterval: time.Millisecond},
			client: &testutil.FakeClient{
				GrantFunc: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
					return &clientv3.LeaseGrantResponse{ID: 3}, nil
				},
				KeepAliveFunc: func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
					cancel()
					return nil, errors.New("keepalive failed")
				},
			},
			regs:  map[string]registryEntry{},
			close: make(chan struct{}),
			after: testutil.ImmediateAfter,
		}
		reg.keepAliveLoop(ctx, "/key", "value")
	})

	t.Run("put error retries then exits", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reg := &Registry{
			cfg: RegistryConfig{TTL: time.Second, RetryInterval: time.Millisecond},
			client: &testutil.FakeClient{
				GrantFunc: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
					return &clientv3.LeaseGrantResponse{ID: 4}, nil
				},
				KeepAliveFunc: func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
					ch := make(chan *clientv3.LeaseKeepAliveResponse)
					close(ch)
					return ch, nil
				},
				PutFunc: func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
					cancel()
					return nil, errors.New("put failed")
				},
			},
			regs:  map[string]registryEntry{},
			close: make(chan struct{}),
			after: testutil.ImmediateAfter,
		}
		reg.keepAliveLoop(ctx, "/key", "value")
	})

	t.Run("ttl expiry updates lease and retries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var puts int32
		reg := &Registry{
			cfg: RegistryConfig{TTL: time.Second, RetryInterval: time.Millisecond},
			client: &testutil.FakeClient{
				GrantFunc: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
					return &clientv3.LeaseGrantResponse{ID: 9}, nil
				},
				KeepAliveFunc: func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
					ch := make(chan *clientv3.LeaseKeepAliveResponse, 1)
					ch <- &clientv3.LeaseKeepAliveResponse{TTL: 0}
					close(ch)
					return ch, nil
				},
				PutFunc: func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
					atomic.AddInt32(&puts, 1)
					return &clientv3.PutResponse{}, nil
				},
			},
			regs:  map[string]registryEntry{"/key": {}},
			close: make(chan struct{}),
			after: func(time.Duration) <-chan time.Time { cancel(); return testutil.ImmediateAfter(0) },
		}
		reg.keepAliveLoop(ctx, "/key", "value")
		if atomic.LoadInt32(&puts) == 0 {
			t.Fatal("expected put to be called")
		}
		if reg.regs["/key"].lease != clientv3.LeaseID(9) {
			t.Fatalf("lease = %v, want 9", reg.regs["/key"].lease)
		}
	})
}
