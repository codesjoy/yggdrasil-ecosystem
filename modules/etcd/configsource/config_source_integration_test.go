//go:build integration
// +build integration

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

package configsource

import (
	"context"
	"testing"
	"time"

	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/testutil"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestConfigSourceBlobReadIntegration(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	cli, err := internalclient.New(internalclient.Config{Endpoints: []string{ee.Endpoint}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Put(ctx, "/test/config", "foo: bar"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	src, err := NewConfigSource(Config{Key: "/test/config", Mode: ModeBlob})
	if err != nil {
		t.Fatalf("NewConfigSource() error = %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	var out map[string]any
	if err := data.Unmarshal(&out); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if out["foo"] != "bar" {
		t.Fatalf("expected foo=bar, got %#v", out)
	}
}

func TestConfigSourceKVReadIntegration(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	cli, err := internalclient.New(internalclient.Config{Endpoints: []string{ee.Endpoint}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ops := []clientv3.Op{
		clientv3.OpPut("/test/config/a", "1"),
		clientv3.OpPut("/test/config/b", "2"),
		clientv3.OpPut("/test/config/c/d", "3"),
	}
	if _, err := cli.Txn(ctx).Then(ops...).Commit(); err != nil {
		t.Fatalf("Txn() error = %v", err)
	}

	src, err := NewConfigSource(Config{Prefix: "/test/config", Mode: ModeKV})
	if err != nil {
		t.Fatalf("NewConfigSource() error = %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	var out map[string]any
	if err := data.Unmarshal(&out); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if out["a"] != 1 || out["b"] != 2 {
		t.Fatalf("flat kv data = %#v", out)
	}
	child, ok := out["c"].(map[string]any)
	if !ok || child["d"] != 3 {
		t.Fatalf("expected c.d=3, got %#v", out["c"])
	}
}

func TestConfigSourceWatchIntegration(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	cli, err := internalclient.New(internalclient.Config{Endpoints: []string{ee.Endpoint}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	src, err := NewConfigSource(Config{
		Key:   "/test/config/watch",
		Mode:  ModeBlob,
		Watch: testutil.BoolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewConfigSource() error = %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	watchable, ok := src.(source.Watchable)
	if !ok {
		t.Fatalf("source type %T does not implement Watchable", src)
	}
	ch, err := watchable.Watch()
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = cli.Put(ctx, "/test/config/watch", "updated")
	}()

	select {
	case data := <-ch:
		if string(data.Bytes()) != "updated" {
			t.Fatalf("expected updated, got %q", string(data.Bytes()))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for watch event")
	}
}
