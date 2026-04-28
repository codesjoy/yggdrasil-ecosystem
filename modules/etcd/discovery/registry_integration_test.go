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

package discovery

import (
	"context"
	"testing"
	"time"

	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/testutil"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
)

func TestRegistryRegisterDeregister(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	reg, err := NewRegistry(RegistryConfig{Prefix: "/yggdrasil/registry", TTL: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	inst := testutil.DemoInstance{
		NamespaceValue: "default",
		NameValue:      "svc",
		VersionValue:   "1.0.0",
		EndpointsValue: []yregistry.Endpoint{
			testutil.DemoEndpoint{SchemeValue: "grpc", AddressValue: "127.0.0.1:9000"},
		},
	}

	key, _, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	getCtx, getCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer getCancel()
	resp, err := reg.cli.Get(getCtx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(resp.Kvs) != 1 {
		t.Fatalf("expected key exists, got %d", len(resp.Kvs))
	}

	if err := reg.Deregister(ctx, inst); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	resp, err = reg.cli.Get(getCtx, key)
	if err != nil {
		t.Fatalf("Get() after deregister error = %v", err)
	}
	if len(resp.Kvs) != 0 {
		t.Fatalf("expected key removed, got %d", len(resp.Kvs))
	}
}

func TestRegistryKeepAlive(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	reg, err := NewRegistry(RegistryConfig{
		Prefix:        "/yggdrasil/registry",
		TTL:           3 * time.Second,
		KeepAlive:     testutil.BoolPtr(true),
		RetryInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	inst := testutil.DemoInstance{
		NamespaceValue: "default",
		NameValue:      "svc",
		VersionValue:   "1.0.0",
		EndpointsValue: []yregistry.Endpoint{
			testutil.DemoEndpoint{SchemeValue: "grpc", AddressValue: "127.0.0.1:9000"},
		},
	}

	key, _, err := reg.buildKeyValue(inst)
	if err != nil {
		t.Fatalf("buildKeyValue() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	time.Sleep(5 * time.Second)

	getCtx, getCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer getCancel()
	resp, err := reg.cli.Get(getCtx, key)
	if err != nil {
		t.Fatalf("Get() after keepalive error = %v", err)
	}
	if len(resp.Kvs) != 1 {
		t.Fatalf("expected key still exists, got %d", len(resp.Kvs))
	}
}
