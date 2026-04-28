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
	"os"
	"path/filepath"
	"testing"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
)

func TestModuleExposesCapabilitiesAndBuilders(t *testing.T) {
	mod, ok := Module().(*etcdModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *etcdModule", Module())
	}

	view := config.NewView("yggdrasil", config.NewSnapshot(map[string]any{
		"etcd": map[string]any{
			"clients": map[string]any{
				"default": map[string]any{
					"endpoints":    []any{"127.0.0.1:2379"},
					"dial_timeout": "2s",
				},
			},
		},
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"edge": map[string]any{
					"type": "etcd",
					"config": map[string]any{
						"prefix":    "/custom/registry",
						"namespace": "edge",
						"protocols": []any{"grpc"},
						"debounce":  "200ms",
					},
				},
			},
		},
	}))
	if err := mod.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	caps := mod.Capabilities()
	if len(caps) != 2 {
		t.Fatalf("Capabilities() = %d, want 2", len(caps))
	}
	want := map[string]bool{
		capabilities.RegistryProviderSpec.Name + "/etcd": false,
		capabilities.ResolverProviderSpec.Name + "/etcd": false,
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

	cfg := mod.clientConfig("default")
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0] != "127.0.0.1:2379" {
		t.Fatalf("clientConfig(default) = %#v", cfg)
	}
	if cfg.DialTimeout.String() != "2s" {
		t.Fatalf("clientConfig(default).DialTimeout = %v, want 2s", cfg.DialTimeout)
	}

	resolverCfg := mod.resolverConfig("edge")
	if resolverCfg.Prefix != "/custom/registry" || resolverCfg.Namespace != "edge" {
		t.Fatalf("resolverConfig(edge) = %#v", resolverCfg)
	}
	if len(resolverCfg.Protocols) != 1 || resolverCfg.Protocols[0] != "grpc" {
		t.Fatalf("resolverConfig(edge).Protocols = %#v", resolverCfg.Protocols)
	}
}

func TestModuleConfigSourceBuilder(t *testing.T) {
	mod := Module().(*etcdModule)
	builders := mod.ConfigSourceBuilders()
	builder := builders[ConfigSourceKind]
	if builder == nil {
		t.Fatal("etcd config source builder not exposed")
	}

	src, priority, err := builder(
		configchain.BuildContext{
			Snapshot: config.NewSnapshot(map[string]any{
				"yggdrasil": map[string]any{
					"etcd": map[string]any{
						"clients": map[string]any{
							"default": map[string]any{
								"endpoints":    []any{"127.0.0.1:2379"},
								"dial_timeout": "3s",
							},
						},
					},
				},
			}),
		},
		configchain.SourceSpec{
			Kind:     ConfigSourceKind,
			Priority: "remote",
			Config: map[string]any{
				"client": "default",
				"key":    "/app/config.yaml",
				"watch":  true,
			},
		},
	)
	if err != nil {
		t.Fatalf("builder() error = %v", err)
	}
	if priority != config.PriorityRemote {
		t.Fatalf("priority = %v, want %v", priority, config.PriorityRemote)
	}
	if src.Kind() != ConfigSourceKind || src.Name() != "/app/config.yaml" {
		t.Fatalf("source identity = kind:%s name:%s", src.Kind(), src.Name())
	}
}

func TestWithModuleCreatesApp(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
yggdrasil:
  etcd:
    clients:
      default:
        endpoints: ["127.0.0.1:2379"]
        dial_timeout: 2s
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	app, err := yggdrasil.New(
		"etcd-module-test",
		yggdrasil.WithConfigPath(cfgPath),
		WithModule(),
	)
	if err != nil {
		t.Fatalf("yggdrasil.New() error = %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}
