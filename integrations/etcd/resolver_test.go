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

package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestLoadResolverConfig(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_etcd_resolver"
	defer func() { config.KeyBase = origKeyBase }()

	if err := config.Set(config.Join(config.KeyBase, "resolver", "demo", "config"), map[string]any{
		"prefix":    "/prefix",
		"namespace": "ns",
		"protocols": []string{"grpc"},
		"debounce":  "250ms",
	}); err != nil {
		t.Fatalf("config.Set: %v", err)
	}

	cfg := LoadResolverConfig("demo")
	if cfg.Prefix != "/prefix" || cfg.Namespace != "ns" || len(cfg.Protocols) != 1 ||
		cfg.Protocols[0] != "grpc" ||
		cfg.Debounce != 250*time.Millisecond {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestNewResolverTypeAndHelpers(t *testing.T) {
	res, err := NewResolver("demo", ResolverConfig{})
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer func() { _ = res.client.Close() }()

	if res.Type() != "etcd" {
		t.Fatalf("Type = %q, want etcd", res.Type())
	}
	if got := res.servicePrefix("svc"); got != "/default/svc" {
		t.Fatalf("servicePrefix = %q, want /default/svc", got)
	}
	if got := instanceKey("default", "svc", "v1", "grpc", "127.0.0.1:9000"); got != "default/svc/v1/grpc/127.0.0.1:9000" {
		t.Fatalf("instanceKey = %q", got)
	}
	if allow := toProtocolAllow(nil); !allow["grpc"] || !allow["http"] {
		t.Fatalf("default allow = %#v", allow)
	}
	if allow := toProtocolAllow([]string{"grpc"}); allow["http"] || !allow["grpc"] {
		t.Fatalf("custom allow = %#v", allow)
	}
}

func TestResolverSnapshotFetchAndNotify(t *testing.T) {
	record := instanceRecord{
		Namespace: "default",
		Name:      "svc",
		Version:   "v1",
		Region:    "cn",
		Zone:      "hz-a",
		Campus:    "campus-a",
		Metadata:  map[string]string{"app": "demo", "shared": "instance"},
		Endpoints: []endpointRecord{
			{
				Scheme:   "grpc",
				Address:  "127.0.0.1:9001",
				Metadata: map[string]string{"shared": "endpoint", "id": "b"},
			},
			{Scheme: "grpc", Address: "127.0.0.1:9000", Metadata: map[string]string{"id": "a"}},
			{Scheme: "http", Address: "127.0.0.1:8080"},
			{Scheme: "grpc", Address: "127.0.0.1:9000", Metadata: map[string]string{"id": "dup"}},
		},
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	res := &Resolver{
		cfg: ResolverConfig{Prefix: "/registry", Protocols: []string{"grpc"}},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return getResp(7,
					kv("/registry/default/svc/1", string(payload)),
					kv("/registry/default/svc/2", "not-json"),
					kv("/registry/default/svc/3", `{"namespace":"default","name":"other"}`),
				), nil
			},
		},
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}

	state, err := res.fetchState(context.Background(), "svc")
	if err != nil {
		t.Fatalf("fetchState: %v", err)
	}
	attrs := state.GetAttributes()
	if attrs["revision"].(int64) != 7 || attrs["namespace"] != "default" {
		t.Fatalf("attributes = %#v", attrs)
	}
	eps := state.GetEndpoints()
	if len(eps) != 2 {
		t.Fatalf("endpoints len = %d, want 2", len(eps))
	}
	if eps[0].GetAddress() != "127.0.0.1:9000" || eps[1].GetAddress() != "127.0.0.1:9001" {
		t.Fatalf("sorted endpoints = %#v", eps)
	}
	if got := eps[1].GetAttributes()["shared"]; got != "endpoint" {
		t.Fatalf("endpoint metadata should override instance metadata, got %#v", got)
	}

	w1 := newCaptureWatcher(1)
	w2 := newCaptureWatcher(1)
	res.watchers["svc"] = map[yresolver.Client]struct{}{w1: {}, w2: {}}
	res.fetchAndNotify(context.Background(), "svc")
	_ = mustReceiveState(t, w1.ch)
	_ = mustReceiveState(t, w2.ch)

	copied := res.snapshotWatchers("svc")
	delete(res.watchers["svc"], w1)
	if len(copied) != 2 {
		t.Fatalf("snapshotWatchers len = %d, want 2", len(copied))
	}
}

func TestResolverFetchStateErrorAndNamespaceFiltering(t *testing.T) {
	wantErr := errors.New("get failed")
	res := &Resolver{
		cfg: ResolverConfig{Prefix: "/registry", Namespace: "ns-a", Protocols: []string{"grpc"}},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return nil, wantErr
			},
		},
	}
	if _, err := res.fetchState(context.Background(), "svc"); !errors.Is(err, wantErr) {
		t.Fatalf("fetchState error = %v, want %v", err, wantErr)
	}

	recordA, _ := json.Marshal(
		instanceRecord{
			Name:      "svc",
			Namespace: "ns-a",
			Endpoints: []endpointRecord{{Scheme: "grpc", Address: "a:1"}},
		},
	)
	recordB, _ := json.Marshal(
		instanceRecord{
			Name:      "svc",
			Namespace: "ns-b",
			Endpoints: []endpointRecord{{Scheme: "grpc", Address: "b:1"}},
		},
	)
	res.client = &fakeEtcdClient{
		getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
			return getResp(
				3,
				kv("/registry/ns-a/svc/1", string(recordA)),
				kv("/registry/ns-b/svc/2", string(recordB)),
			), nil
		},
	}
	state, err := res.fetchState(context.Background(), "svc")
	if err != nil {
		t.Fatalf("fetchState filtered: %v", err)
	}
	if len(state.GetEndpoints()) != 1 || state.GetEndpoints()[0].GetAddress() != "a:1" {
		t.Fatalf("filtered endpoints = %#v", state.GetEndpoints())
	}
}

func TestResolverAddAndDelWatch(t *testing.T) {
	watchCh := make(chan clientv3.WatchResponse)
	close(watchCh)
	res := &Resolver{
		cfg: ResolverConfig{Prefix: "/registry"},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return getResp(1), nil
			},
			watchFunc: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return watchCh },
		},
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}
	watcher := newCaptureWatcher(1)
	if err := res.AddWatch("svc", watcher); err != nil {
		t.Fatalf("AddWatch: %v", err)
	}
	if len(res.watchers["svc"]) != 1 {
		t.Fatalf("watchers = %#v, want one watcher", res.watchers)
	}
	if _, ok := res.cancels["svc"]; !ok {
		t.Fatal("expected cancel func to be registered")
	}
	if err := res.DelWatch("svc", watcher); err != nil {
		t.Fatalf("DelWatch: %v", err)
	}
	if _, ok := res.watchers["svc"]; ok {
		t.Fatalf("watchers still contains svc: %#v", res.watchers)
	}
	if _, ok := res.cancels["svc"]; ok {
		t.Fatalf("cancels still contains svc: %#v", res.cancels)
	}
}

func TestResolverWatchLoop(t *testing.T) {
	record, err := json.Marshal(
		instanceRecord{
			Name:      "svc",
			Namespace: "default",
			Endpoints: []endpointRecord{{Scheme: "grpc", Address: "127.0.0.1:9000"}},
		},
	)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	watchCh := make(chan clientv3.WatchResponse, 1)
	client := &fakeEtcdClient{}
	client.getFunc = func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		return getResp(5, kv("/registry/default/svc/1", string(record))), nil
	}
	client.watchFunc = func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return watchCh }

	watcher := newCaptureWatcher(2)
	res := &Resolver{
		cfg:      ResolverConfig{Prefix: "/registry", Debounce: 0},
		client:   client,
		watchers: map[string]map[yresolver.Client]struct{}{"svc": {watcher: {}}},
		cancels:  map[string]context.CancelFunc{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go res.watchLoop(ctx, "svc")

	state := mustReceiveState(t, watcher.ch)
	if len(state.GetEndpoints()) != 1 {
		t.Fatalf("initial endpoints = %#v", state.GetEndpoints())
	}
	watchCh <- clientv3.WatchResponse{}
	_ = mustReceiveState(t, watcher.ch)
	cancel()
	close(watchCh)
}

func TestResolverFetchAndNotifySkipsErrors(t *testing.T) {
	watcher := newCaptureWatcher(1)
	res := &Resolver{
		cfg: ResolverConfig{Prefix: "/registry"},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return nil, fmt.Errorf("boom")
			},
		},
		watchers: map[string]map[yresolver.Client]struct{}{"svc": {watcher: {}}},
		cancels:  map[string]context.CancelFunc{},
	}
	res.fetchAndNotify(context.Background(), "svc")
	mustNotReceiveState(t, watcher.ch)
}
