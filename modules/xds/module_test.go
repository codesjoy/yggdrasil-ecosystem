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

package xds

import (
	"context"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v3/config"
)

func TestModuleCapabilitiesAndConfig(t *testing.T) {
	mod := Module()
	if mod.Name() != "xds" {
		t.Fatalf("Name() = %q, want xds", mod.Name())
	}
	capProvider := mod.(*xdsModule)
	if len(capProvider.Capabilities()) != 2 {
		t.Fatalf("Capabilities() = %d, want 2", len(capProvider.Capabilities()))
	}

	view := config.NewView("yggdrasil", config.NewSnapshot(map[string]any{
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"xds-test": map[string]any{
					"type": "xds",
					"config": map[string]any{
						"name": "cfg",
					},
				},
			},
		},
		"xds": map[string]any{
			"cfg": map[string]any{
				"config": map[string]any{
					"server": map[string]any{
						"address": "127.0.0.1:19001",
						"timeout": "2s",
					},
					"service_map": map[string]any{"svc": "listener-1"},
				},
			},
		},
	}))
	if err := capProvider.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfg := capProvider.resolverConfig("xds-test")
	if cfg.Server.Address != "127.0.0.1:19001" || cfg.Server.Timeout != 2*time.Second {
		t.Fatalf("unexpected module resolver config: %#v", cfg)
	}
	if cfg.ServiceMap["svc"] != "listener-1" {
		t.Fatalf("unexpected service map: %#v", cfg.ServiceMap)
	}
}
