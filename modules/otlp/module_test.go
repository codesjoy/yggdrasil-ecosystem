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

package otlp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
)

func TestModuleConfig(t *testing.T) {
	mod, ok := Module().(*otlpModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *otlpModule", Module())
	}

	if mod.Name() != capabilityName {
		t.Fatalf("Name() = %q, want %q", mod.Name(), capabilityName)
	}
	if mod.ConfigPath() != "yggdrasil.observability.telemetry.providers.otlp" {
		t.Fatalf("ConfigPath() = %q, want v3 otlp provider path", mod.ConfigPath())
	}

	view := config.NewView(mod.ConfigPath(), config.NewSnapshot(map[string]any{
		"trace": map[string]any{
			"endpoint": "collector:4317",
			"headers":  map[string]any{"authorization": "token"},
		},
		"metric": map[string]any{
			"endpoint":       "collector:4318",
			"exportInterval": "10s",
		},
	}))
	if err := mod.Init(context.Background(), view); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if got := mod.traceConfig().Endpoint; got != "collector:4317" {
		t.Fatalf("trace endpoint = %q, want collector:4317", got)
	}
	if got := mod.traceConfig().Headers["authorization"]; got != "token" {
		t.Fatalf("trace authorization header = %q, want token", got)
	}
	if got := mod.metricConfig().Endpoint; got != "collector:4318" {
		t.Fatalf("metric endpoint = %q, want collector:4318", got)
	}
}

func TestModuleExposesV3Capabilities(t *testing.T) {
	mod, ok := Module().(*otlpModule)
	if !ok {
		t.Fatalf("Module() type = %T, want *otlpModule", Module())
	}

	want := map[string]bool{
		capabilities.TracerProviderSpec.Name + "/" + grpcProviderName: false,
		capabilities.TracerProviderSpec.Name + "/" + httpProviderName: false,
		capabilities.MeterProviderSpec.Name + "/" + grpcProviderName:  false,
		capabilities.MeterProviderSpec.Name + "/" + httpProviderName:  false,
	}
	for _, cap := range mod.Capabilities() {
		switch cap.Spec.Name {
		case capabilities.TracerProviderSpec.Name:
			if _, ok := cap.Value.(xotel.TracerProviderBuilder); !ok {
				t.Fatalf("trace provider %q type = %T", cap.Name, cap.Value)
			}
		case capabilities.MeterProviderSpec.Name:
			if _, ok := cap.Value.(xotel.MeterProviderBuilder); !ok {
				t.Fatalf("meter provider %q type = %T", cap.Name, cap.Value)
			}
		}
		key := cap.Spec.Name + "/" + cap.Name
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("capability %s not exposed; got %#v", key, mod.Capabilities())
		}
	}
}

func TestWithModule(t *testing.T) {
	if WithModule() == nil {
		t.Fatal("WithModule() = nil")
	}

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
yggdrasil:
  observability:
    telemetry:
      tracer: otlp-grpc
      meter: otlp-http
      providers:
        otlp:
          trace:
            endpoint: localhost:4317
            tls:
              insecure: true
          metric:
            endpoint: localhost:4318
            tls:
              insecure: true
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	app, err := yggdrasil.New(
		"otlp-test",
		yggdrasil.WithConfigPath(cfgPath),
		WithModule(),
	)
	if err != nil {
		t.Fatalf("yggdrasil.New() error = %v", err)
	}
	if app == nil {
		t.Fatal("yggdrasil.New() app = nil")
	}
}
