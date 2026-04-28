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
	"sync"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/module"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
)

const (
	capabilityName = "otlp"

	grpcProviderName = "otlp-grpc"
	httpProviderName = "otlp-http"
)

type otlpModule struct {
	mu       sync.RWMutex
	settings Config
}

// Module returns the Yggdrasil v3 OTLP provider module.
func Module() module.Module {
	return &otlpModule{}
}

// WithModule registers the OTLP provider module on a Yggdrasil v3 app.
func WithModule() yggdrasil.Option {
	return yggdrasil.WithModules(Module())
}

func (m *otlpModule) Name() string { return capabilityName }

func (m *otlpModule) ConfigPath() string {
	return "yggdrasil.observability.telemetry.providers.otlp"
}

func (m *otlpModule) Init(_ context.Context, view config.View) error {
	var next Config
	if view.Exists() {
		if err := view.Decode(&next); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.settings = next
	m.mu.Unlock()
	return nil
}

func (m *otlpModule) Capabilities() []module.Capability {
	return []module.Capability{
		capabilities.ProvideNamed(
			capabilities.TracerProviderSpec,
			grpcProviderName,
			xotel.TracerProviderBuilder(m.newGRPCTracerProvider),
		),
		capabilities.ProvideNamed(
			capabilities.TracerProviderSpec,
			httpProviderName,
			xotel.TracerProviderBuilder(m.newHTTPTracerProvider),
		),
		capabilities.ProvideNamed(
			capabilities.MeterProviderSpec,
			grpcProviderName,
			xotel.MeterProviderBuilder(m.newGRPCMeterProvider),
		),
		capabilities.ProvideNamed(
			capabilities.MeterProviderSpec,
			httpProviderName,
			xotel.MeterProviderBuilder(m.newHTTPMeterProvider),
		),
	}
}

func (m *otlpModule) traceConfig() TraceExporterConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneTraceConfig(m.settings.Trace)
}

func (m *otlpModule) metricConfig() MetricExporterConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneMetricConfig(m.settings.Metric)
}

func cloneTraceConfig(in TraceExporterConfig) TraceExporterConfig {
	in.Headers = cloneStringMap(in.Headers)
	in.Resource = cloneAnyMap(in.Resource)
	return in
}

func cloneMetricConfig(in MetricExporterConfig) MetricExporterConfig {
	in.Headers = cloneStringMap(in.Headers)
	in.Resource = cloneAnyMap(in.Resource)
	return in
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneAnyMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
