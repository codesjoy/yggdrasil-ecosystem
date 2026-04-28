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

package k8s

import (
	"context"
	"testing"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
)

func TestModuleExposesResolverCapabilityAndBuilders(t *testing.T) {
	mod, ok := Module().(*k8sModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *k8sModule", Module())
	}

	view := config.NewView("yggdrasil", config.NewSnapshot(map[string]any{
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"k8s": map[string]any{
					"type": "kubernetes",
					"config": map[string]any{
						"protocol": "http",
						"backoff": map[string]any{
							"base_delay": "2s",
						},
					},
				},
			},
		},
	}))
	if err := mod.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	caps := mod.Capabilities()
	if len(caps) != 1 {
		t.Fatalf("Capabilities() = %d, want 1", len(caps))
	}
	if caps[0].Spec.Name != capabilities.ResolverProviderSpec.Name || caps[0].Name != "kubernetes" {
		t.Fatalf("unexpected capability: %#v", caps[0])
	}

	cfg := mod.resolverConfig("k8s")
	if cfg.Protocol != "http" {
		t.Fatalf("resolverConfig.Protocol = %q, want http", cfg.Protocol)
	}
	if cfg.Backoff.BaseDelay.String() != "2s" {
		t.Fatalf("resolverConfig.Backoff.BaseDelay = %v, want 2s", cfg.Backoff.BaseDelay)
	}
	if cfg.Mode != "endpointslice" {
		t.Fatalf("resolverConfig.Mode = %q, want endpointslice", cfg.Mode)
	}

	builders := mod.ConfigSourceBuilders()
	if builders["kubernetes-configmap"] == nil || builders["kubernetes-secret"] == nil {
		t.Fatalf("ConfigSourceBuilders() missing kubernetes builders: %#v", builders)
	}
}

func TestModuleConfigSourceBuilders(t *testing.T) {
	mod := Module().(*k8sModule)
	builders := mod.ConfigSourceBuilders()

	configMapSource, priority, err := builders["kubernetes-configmap"](
		configchain.BuildContext{},
		configchain.SourceSpec{
			Kind:     "kubernetes-configmap",
			Priority: "remote",
			Config: map[string]any{
				"namespace": "default",
				"name":      "app-config",
				"key":       "config.yaml",
				"watch":     true,
			},
		},
	)
	if err != nil {
		t.Fatalf("configmap builder() error = %v", err)
	}
	if priority != config.PriorityRemote {
		t.Fatalf("priority = %v, want %v", priority, config.PriorityRemote)
	}
	if configMapSource.Kind() != "kubernetes-configmap" || configMapSource.Name() != "app-config" {
		t.Fatalf(
			"unexpected configmap source: kind=%s name=%s",
			configMapSource.Kind(),
			configMapSource.Name(),
		)
	}

	secretSource, priority, err := builders["kubernetes-secret"](
		configchain.BuildContext{},
		configchain.SourceSpec{
			Kind:     "kubernetes-secret",
			Priority: "remote",
			Config: map[string]any{
				"namespace": "default",
				"name":      "app-secret",
			},
		},
	)
	if err != nil {
		t.Fatalf("secret builder() error = %v", err)
	}
	if priority != config.PriorityRemote {
		t.Fatalf("priority = %v, want %v", priority, config.PriorityRemote)
	}
	if secretSource.Kind() != "kubernetes-secret" || secretSource.Name() != "app-secret" {
		t.Fatalf(
			"unexpected secret source: kind=%s name=%s",
			secretSource.Kind(),
			secretSource.Name(),
		)
	}
}

func TestRootConfigSourceHelpers(t *testing.T) {
	configMapSource, err := NewConfigMapSource(ConfigSourceConfig{
		Namespace: "default",
		Name:      "app-config",
	})
	if err != nil {
		t.Fatalf("NewConfigMapSource() error = %v", err)
	}
	if configMapSource.Kind() != "kubernetes-configmap" || configMapSource.Name() != "app-config" {
		t.Fatalf(
			"unexpected configmap source identity: kind=%s name=%s",
			configMapSource.Kind(),
			configMapSource.Name(),
		)
	}

	secretSource, err := NewSecretSource(ConfigSourceConfig{
		Namespace: "default",
		Name:      "app-secret",
	})
	if err != nil {
		t.Fatalf("NewSecretSource() error = %v", err)
	}
	if secretSource.Kind() != "kubernetes-secret" || secretSource.Name() != "app-secret" {
		t.Fatalf(
			"unexpected secret source identity: kind=%s name=%s",
			secretSource.Kind(),
			secretSource.Name(),
		)
	}
}

func TestWithModuleCreatesApp(t *testing.T) {
	app, err := yapp.New("test-with-module", yapp.WithModules(Module()))
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}
