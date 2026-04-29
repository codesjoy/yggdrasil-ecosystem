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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3/configsource"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3/discovery"
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

func TestModuleHelperFunctionsAndFallbacks(t *testing.T) {
	mod := Module().(*k8sModule)
	if mod.Name() != moduleName {
		t.Fatalf("Name() = %q, want %q", mod.Name(), moduleName)
	}
	if mod.ConfigPath() != "yggdrasil" {
		t.Fatalf("ConfigPath() = %q, want yggdrasil", mod.ConfigPath())
	}
	if WithModule() == nil {
		t.Fatal("WithModule() returned nil option")
	}
	if WithConfigMapSource("", config.PriorityRemote, ConfigSourceConfig{}) == nil {
		t.Fatal("WithConfigMapSource() returned nil option")
	}
	if WithSecretSource("", config.PriorityRemote, ConfigSourceConfig{}) == nil {
		t.Fatal("WithSecretSource() returned nil option")
	}

	sentinel := errors.New("boom")
	src := invalidSource{
		kind: configsource.KindConfigMap,
		name: "fallback",
		err:  sentinel,
	}
	if src.Kind() != configsource.KindConfigMap {
		t.Fatalf("Kind() = %q, want %q", src.Kind(), configsource.KindConfigMap)
	}
	if src.Name() != "fallback" {
		t.Fatalf("Name() = %q, want fallback", src.Name())
	}
	if _, err := src.Read(); !errors.Is(err, sentinel) {
		t.Fatalf("Read() error = %v, want %v", err, sentinel)
	}
	if err := src.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := mod.Init(context.Background(), config.NewView("", config.NewSnapshot(map[string]any{}))); err != nil {
		t.Fatalf("Init() with empty view error = %v", err)
	}
	badView := config.NewView("yggdrasil", config.NewSnapshot(map[string]any{
		"discovery": "bad",
	}))
	if err := mod.Init(context.Background(), badView); err == nil {
		t.Fatal("Init() expected decode error")
	}
}

func TestFallbackSourceName(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		fallback string
		want     string
	}{
		{name: "explicit", explicit: "primary", fallback: "secondary", want: "primary"},
		{name: "fallback", explicit: " ", fallback: "secondary", want: "secondary"},
		{name: "module default", explicit: "", fallback: "", want: moduleName},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fallbackSourceName(test.explicit, test.fallback); got != test.want {
				t.Fatalf("fallbackSourceName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestModuleBuildConfigSourceErrorPaths(t *testing.T) {
	mod := Module().(*k8sModule)

	if _, _, err := mod.buildConfigSource(
		configsource.KindConfigMap,
		configchain.SourceSpec{
			Priority: "remote",
			Config: map[string]any{
				"name":   "bad-parser",
				"format": "bogus",
			},
		},
	); err == nil {
		t.Fatal("buildConfigSource() expected decode error")
	}

	if _, _, err := mod.buildConfigSource(
		configsource.KindConfigMap,
		configchain.SourceSpec{
			Priority: "bogus",
			Config: map[string]any{
				"name": "app-config",
			},
		},
	); err == nil {
		t.Fatal("buildConfigSource() expected invalid priority error")
	}

	if _, _, err := mod.buildConfigSource(
		"unsupported",
		configchain.SourceSpec{
			Priority: "remote",
			Config: map[string]any{
				"name": "app-config",
			},
		},
	); err == nil || !strings.Contains(err.Error(), "unsupported config source kind") {
		t.Fatalf("buildConfigSource() error = %v, want unsupported kind", err)
	}
}

func TestDecodeMapAndResolverConfigFallback(t *testing.T) {
	var cfg discovery.ResolverConfig
	if err := decodeMap(map[string]any{
		"timeout": "3s",
		"backoff": map[string]any{
			"base_delay": "2s",
		},
	}, &cfg); err != nil {
		t.Fatalf("decodeMap() error = %v", err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Fatalf("decodeMap Timeout = %v, want 3s", cfg.Timeout)
	}
	if cfg.Backoff.BaseDelay != 2*time.Second {
		t.Fatalf("decodeMap BaseDelay = %v, want 2s", cfg.Backoff.BaseDelay)
	}

	if err := decodeMap(map[string]any{
		"backoff": map[string]any{
			"base_delay": "not-a-duration",
		},
	}, &cfg); err == nil {
		t.Fatal("decodeMap() expected duration decode error")
	}

	t.Setenv("KUBERNETES_NAMESPACE", "")
	mod := Module().(*k8sModule)
	mod.settings.Discovery.Resolvers = map[string]struct {
		Type   string         `mapstructure:"type"`
		Config map[string]any `mapstructure:"config"`
	}{
		"broken": {
			Type: "kubernetes",
			Config: map[string]any{
				"port": "not-a-number",
			},
		},
	}

	resolved := mod.resolverConfig("broken")
	if resolved.Namespace != "default" {
		t.Fatalf("resolverConfig.Namespace = %q, want default", resolved.Namespace)
	}
	if resolved.Mode != "endpointslice" {
		t.Fatalf("resolverConfig.Mode = %q, want endpointslice", resolved.Mode)
	}
	if resolved.Protocol != "grpc" {
		t.Fatalf("resolverConfig.Protocol = %q, want grpc", resolved.Protocol)
	}
}
