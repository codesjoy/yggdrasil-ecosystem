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

import "testing"

func TestPublicResolverWrappers(t *testing.T) {
	cfg := DefaultResolverConfig()
	if cfg.Server.Address == "" || cfg.Protocol == "" {
		t.Fatalf("DefaultResolverConfig() = %#v", cfg)
	}

	resolver, err := NewResolver("svc", cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	if resolver.Type() != "xds" {
		t.Fatalf("resolver.Type() = %q, want xds", resolver.Type())
	}

	var loaded string
	provider := ResolverProvider(func(name string) ResolverConfig {
		loaded = name
		return cfg
	})
	if provider.Type() != "xds" {
		t.Fatalf("provider.Type() = %q, want xds", provider.Type())
	}

	built, err := provider.New("svc")
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if loaded != "svc" {
		t.Fatalf("loader called with %q, want svc", loaded)
	}
	if built.Type() != "xds" {
		t.Fatalf("built.Type() = %q, want xds", built.Type())
	}
}

func TestResolverProviderWithDefaultLoader(t *testing.T) {
	provider := ResolverProvider(nil)
	resolver, err := provider.New("default")
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if resolver.Type() != "xds" {
		t.Fatalf("resolver.Type() = %q, want xds", resolver.Type())
	}
}
