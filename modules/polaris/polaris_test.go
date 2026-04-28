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
	"testing"

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
