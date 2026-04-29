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

package polaris

import (
	"context"
	"errors"
	"testing"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/internal/sdk"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
)

func TestModuleExposesV3Capabilities(t *testing.T) {
	mod, ok := Module().(*polarisModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *polarisModule", Module())
	}

	view := config.NewView("yggdrasil", config.NewSnapshot(map[string]any{
		"polaris": map[string]any{
			"sdks": map[string]any{
				"default": map[string]any{"addresses": []any{"127.0.0.1:8091"}},
			},
		},
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"svc": map[string]any{
					"type": "polaris",
					"config": map[string]any{
						"namespace":        "default",
						"refresh_interval": "1s",
					},
				},
			},
		},
	}))
	if err := mod.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	caps := mod.Capabilities()
	want := map[string]bool{
		capabilities.RegistryProviderSpec.Name + "/polaris":                      false,
		capabilities.ResolverProviderSpec.Name + "/polaris":                      false,
		capabilities.BalancerProviderSpec.Name + "/polaris":                      false,
		capabilities.UnaryClientInterceptorSpec.Name + "/polaris_ratelimit":      false,
		capabilities.UnaryClientInterceptorSpec.Name + "/polaris_circuitbreaker": false,
	}
	for _, cap := range caps {
		key := cap.Spec.Name + "/" + cap.Name
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("capability %s not exposed; got %#v", key, caps)
		}
	}

	if cfg := mod.sdkConfig("default"); len(cfg.Addresses) != 1 ||
		cfg.Addresses[0] != "127.0.0.1:8091" {
		t.Fatalf("sdkConfig(default) = %#v", cfg)
	}
	if cfg := mod.resolverConfig("svc"); cfg.Namespace != "default" || cfg.RefreshInterval == 0 {
		t.Fatalf("resolverConfig(svc) = %#v", cfg)
	}
}

func TestModuleConfigSourceBuilderUsesBaseSDKConfig(t *testing.T) {
	mod, ok := Module().(*polarisModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *polarisModule", Module())
	}

	builders := mod.ConfigSourceBuilders()
	builder := builders["polaris"]
	if builder == nil {
		t.Fatal("polaris config source builder not exposed")
	}
	src, priority, err := builder(
		configchain.BuildContext{
			Snapshot: config.NewSnapshot(map[string]any{
				"yggdrasil": map[string]any{
					"polaris": map[string]any{
						"sdks": map[string]any{
							"default": map[string]any{
								"addresses":        []any{"127.0.0.1:8091"},
								"config_addresses": []any{"127.0.0.1:8093"},
								"token":            "token",
								"config_file":      "/tmp/polaris.yaml",
							},
						},
					},
				},
			}),
		},
		configchain.SourceSpec{
			Kind:     "polaris",
			Priority: "remote",
			Config: map[string]any{
				"sdk":           "default",
				"namespace":     "default",
				"file_group":    "yggdrasil",
				"file_name":     "application.yaml",
				"fetch_timeout": "2s",
			},
		},
	)
	if err != nil {
		t.Fatalf("builder() error = %v", err)
	}
	if priority != config.PriorityRemote {
		t.Fatalf("priority = %v, want %v", priority, config.PriorityRemote)
	}
	if src == nil || src.Kind() != "polaris" || src.Name() != "application.yaml" {
		t.Fatalf("source = %#v", src)
	}
	sdkCfg := mod.sdkConfig("default")
	if len(sdkCfg.ConfigAddress) != 1 || sdkCfg.ConfigAddress[0] != "127.0.0.1:8093" {
		t.Fatalf("sdk config addresses = %#v", sdkCfg.ConfigAddress)
	}
	if sdkCfg.Token != "token" || sdkCfg.ConfigFile != "/tmp/polaris.yaml" {
		t.Fatalf("sdk config = %#v", sdkCfg)
	}
}

func TestModuleIdentityInvalidSourceAndGovernanceMerge(t *testing.T) {
	mod := &polarisModule{
		settings: settings{
			Polaris: struct {
				SDKs       map[string]sdk.Config `mapstructure:"sdks"`
				Governance struct {
					Defaults map[string]any            `mapstructure:"defaults"`
					Services map[string]map[string]any `mapstructure:"services"`
				} `mapstructure:"governance"`
			}{
				Governance: struct {
					Defaults map[string]any            `mapstructure:"defaults"`
					Services map[string]map[string]any `mapstructure:"services"`
				}{
					Defaults: map[string]any{
						"namespace":  "default",
						"rate_limit": map[string]any{"enable": true},
					},
					Services: map[string]map[string]any{"svc": {"caller_service": "caller"}},
				},
			},
			Balancers: struct {
				Defaults map[string]struct {
					Type   string         `mapstructure:"type"`
					Config map[string]any `mapstructure:"config"`
				} `mapstructure:"defaults"`
				Services map[string]map[string]struct {
					Type   string         `mapstructure:"type"`
					Config map[string]any `mapstructure:"config"`
				} `mapstructure:"services"`
			}{
				Defaults: map[string]struct {
					Type   string         `mapstructure:"type"`
					Config map[string]any `mapstructure:"config"`
				}{
					"polaris": {Config: map[string]any{"routing": map[string]any{"enable": true}}},
				},
				Services: map[string]map[string]struct {
					Type   string         `mapstructure:"type"`
					Config map[string]any `mapstructure:"config"`
				}{
					"svc": {
						"polaris": {
							Config: map[string]any{"addresses": []string{"127.0.0.1:8091"}},
						},
					},
				},
			},
		},
	}

	if mod.Name() != "polaris" {
		t.Fatalf("Name() = %q", mod.Name())
	}
	if mod.ConfigPath() != "yggdrasil" {
		t.Fatalf("ConfigPath() = %q", mod.ConfigPath())
	}

	wantErr := errors.New("invalid source")
	src := invalidSource{name: "remote", err: wantErr}
	if src.Kind() != "polaris" || src.Name() != "remote" {
		t.Fatalf("invalidSource identity = (%s, %s)", src.Kind(), src.Name())
	}
	if data, err := src.Read(); data != nil || !errors.Is(err, wantErr) {
		t.Fatalf("invalidSource.Read() = (%#v, %v)", data, err)
	}
	if err := src.Close(); err != nil {
		t.Fatalf("invalidSource.Close() error = %v", err)
	}

	cfg := mod.governanceConfig("svc")
	if cfg["namespace"] != "default" || cfg["caller_service"] != "caller" {
		t.Fatalf("governanceConfig() basics = %#v", cfg)
	}
	routing, ok := cfg["routing"].(map[string]any)
	if !ok || routing["enable"] != true {
		t.Fatalf("governanceConfig() routing = %#v", cfg["routing"])
	}
	addresses, ok := cfg["addresses"].([]string)
	if !ok || len(addresses) != 1 || addresses[0] != "127.0.0.1:8091" {
		t.Fatalf("governanceConfig() addresses = %#v", cfg["addresses"])
	}
}

func TestModuleInitAndConfigSourceBuilderErrors(t *testing.T) {
	mod := &polarisModule{}

	view := config.NewView("yggdrasil", config.NewSnapshot("bad"))
	if err := mod.Init(context.Background(), view); err == nil {
		t.Fatal("Init() should fail for invalid duration")
	}

	builder := mod.ConfigSourceBuilders()["polaris"]
	if builder == nil {
		t.Fatal("polaris config source builder missing")
	}

	_, _, err := builder(
		configchain.BuildContext{
			Snapshot: config.NewSnapshot(map[string]any{"yggdrasil": "bad"}),
		},
		configchain.SourceSpec{
			Kind:   "polaris",
			Config: map[string]any{"file_name": "application.yaml"},
		},
	)
	if err == nil {
		t.Fatal("builder() should fail when snapshot cannot decode")
	}

	_, _, err = builder(
		configchain.BuildContext{},
		configchain.SourceSpec{
			Kind: "polaris",
			Config: map[string]any{
				"file_name":     "application.yaml",
				"fetch_timeout": "bad-duration",
			},
		},
	)
	if err == nil {
		t.Fatal("builder() should fail when spec config cannot decode")
	}

	_, _, err = builder(
		configchain.BuildContext{},
		configchain.SourceSpec{
			Kind:     "polaris",
			Priority: "invalid",
			Config:   map[string]any{"file_name": "application.yaml"},
		},
	)
	if err == nil {
		t.Fatal("builder() should fail for invalid priority")
	}

	_, _, err = builder(
		configchain.BuildContext{},
		configchain.SourceSpec{
			Kind:   "polaris",
			Config: map[string]any{},
		},
	)
	if err == nil {
		t.Fatal("builder() should fail when config source cannot be constructed")
	}
}
