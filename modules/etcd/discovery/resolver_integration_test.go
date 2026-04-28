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

func TestResolverAddWatch(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	res, err := NewResolver("test", ResolverConfig{
		Prefix:    "/yggdrasil/registry",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	t.Cleanup(func() { _ = res.cli.Close() })

	reg, err := NewRegistry(RegistryConfig{
		Prefix: "/yggdrasil/registry",
		TTL:    5 * time.Second,
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := reg.Register(ctx, inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	watcher := testutil.NewCaptureWatcher(1)
	if err := res.AddWatch("svc", watcher); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}

	state := testutil.MustReceiveState(t, watcher.Channel())
	if len(state.GetEndpoints()) == 0 {
		t.Fatal("expected at least one endpoint")
	}
	if state.GetEndpoints()[0].GetAddress() != "127.0.0.1:9000" {
		t.Fatalf("expected address 127.0.0.1:9000, got %s", state.GetEndpoints()[0].GetAddress())
	}
}

func TestResolverDelWatch(t *testing.T) {
	ee := testutil.NewEmbeddedEtcd(t)
	testutil.UseClientConfigs(t, map[string]internalclient.Config{
		internalclient.DefaultClientName: {Endpoints: []string{ee.Endpoint}},
	})

	res, err := NewResolver("test", ResolverConfig{
		Prefix:    "/yggdrasil/registry",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	t.Cleanup(func() { _ = res.cli.Close() })

	watcher := testutil.NewCaptureWatcher(1)
	if err := res.AddWatch("svc", watcher); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}
	if err := res.DelWatch("svc", watcher); err != nil {
		t.Fatalf("DelWatch() error = %v", err)
	}

	select {
	case <-watcher.Channel():
		t.Fatal("expected no state after DelWatch")
	case <-time.After(500 * time.Millisecond):
	}
}
