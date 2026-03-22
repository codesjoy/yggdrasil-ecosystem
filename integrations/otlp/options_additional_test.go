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
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
)

func TestClientOptionBuilders(t *testing.T) {
	traceCfg := TraceExporterConfig{
		Endpoint:    "localhost:4317",
		Headers:     map[string]string{"authorization": "token"},
		Timeout:     time.Second,
		Compression: "gzip",
		TLS:         TLSConfig{Insecure: true},
		Retry: RetryConfig{
			Enabled:      true,
			InitialDelay: time.Millisecond,
			MaxDelay:     5 * time.Millisecond,
			MaxAttempts:  2,
		},
	}
	metricCfg := MetricExporterConfig{
		Endpoint:       "localhost:4318",
		Headers:        map[string]string{"authorization": "token"},
		Timeout:        time.Second,
		Compression:    "gzip",
		TLS:            TLSConfig{Insecure: true},
		ExportInterval: time.Second,
		ExportTimeout:  time.Second,
		Retry: RetryConfig{
			Enabled:      true,
			InitialDelay: time.Millisecond,
			MaxDelay:     5 * time.Millisecond,
			MaxAttempts:  2,
		},
	}

	grpcTraceOpts, err := createGRPCTraceClientOptions(traceCfg)
	if err != nil {
		t.Fatalf("createGRPCTraceClientOptions() error = %v", err)
	}
	if len(grpcTraceOpts) == 0 {
		t.Fatal("createGRPCTraceClientOptions() returned no options")
	}

	httpTraceOpts := createHTTPTraceClientOptions(traceCfg)
	if len(httpTraceOpts) == 0 {
		t.Fatal("createHTTPTraceClientOptions() returned no options")
	}

	grpcMetricOpts, err := createGRPCMeterClientOptions(metricCfg)
	if err != nil {
		t.Fatalf("createGRPCMeterClientOptions() error = %v", err)
	}
	if len(grpcMetricOpts) == 0 {
		t.Fatal("createGRPCMeterClientOptions() returned no options")
	}

	httpMetricOpts := createHTTPMeterClientOptions(metricCfg)
	if len(httpMetricOpts) == 0 {
		t.Fatal("createHTTPMeterClientOptions() returned no options")
	}
}

func TestCreateGRPCDialOptions(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TLSConfig
		wantErr bool
	}{
		{
			name: "insecure",
			cfg:  TLSConfig{Insecure: true},
		},
		{
			name: "tls enabled",
			cfg:  TLSConfig{Enabled: true},
		},
		{
			name: "default insecure fallback",
			cfg:  TLSConfig{},
		},
		{
			name:    "bad ca file",
			cfg:     TLSConfig{Enabled: true, CAFile: "/definitely/missing-ca.pem"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := createGRPCDialOptions(tt.cfg, RetryConfig{})
			if tt.wantErr {
				if err == nil {
					t.Fatal("createGRPCDialOptions() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("createGRPCDialOptions() error = %v", err)
			}
			if len(opts) == 0 {
				t.Fatal("createGRPCDialOptions() returned no options")
			}
		})
	}
}

func TestHTTPClientTLSOptionHelpers(t *testing.T) {
	if opt := createHTTPClientTLSOption(TLSConfig{Insecure: true}); opt == nil {
		t.Fatal("createHTTPClientTLSOption() returned nil for insecure config")
	}
	if opt := createHTTPClientTLSOption(TLSConfig{Enabled: true}); opt == nil {
		t.Fatal("createHTTPClientTLSOption() returned nil for TLS config")
	}
	if opt := createHTTPMetricClientTLSOption(TLSConfig{Insecure: true}); opt == nil {
		t.Fatal("createHTTPMetricClientTLSOption() returned nil for insecure config")
	}
	if opt := createHTTPMetricClientTLSOption(TLSConfig{Enabled: true}); opt == nil {
		t.Fatal("createHTTPMetricClientTLSOption() returned nil for TLS config")
	}
}

func TestExporterFactories(t *testing.T) {
	traceCfg := TraceExporterConfig{
		Endpoint: "localhost:4317",
		TLS:      TLSConfig{Insecure: true},
	}
	metricCfg := MetricExporterConfig{
		Endpoint: "localhost:4317",
		TLS:      TLSConfig{Insecure: true},
	}

	if _, err := createGRPCTraceExporter(context.Background(), traceCfg); err != nil {
		t.Fatalf("createGRPCTraceExporter() error = %v", err)
	}
	if _, err := createHTTPTraceExporter(context.Background(), TraceExporterConfig{
		Endpoint: "localhost:4318",
		TLS:      TLSConfig{Insecure: true},
	}); err != nil {
		t.Fatalf("createHTTPTraceExporter() error = %v", err)
	}
	if _, err := createGRPCMeterExporter(context.Background(), metricCfg); err != nil {
		t.Fatalf("createGRPCMeterExporter() error = %v", err)
	}
	if _, err := createHTTPMeterExporter(context.Background(), MetricExporterConfig{
		Endpoint: "localhost:4318",
		TLS:      TLSConfig{Insecure: true},
	}); err != nil {
		t.Fatalf("createHTTPMeterExporter() error = %v", err)
	}
}

func TestLoadConfigsAndProviderFallbacks(t *testing.T) {
	setConfig := func(t *testing.T, key string, value any) {
		t.Helper()
		if err := config.Set(key, value); err != nil {
			t.Fatalf("config.Set(%q) error = %v", key, err)
		}
	}

	traceBase := config.Join(config.KeyBase, "otlp", "trace")
	setConfig(t, config.Join(traceBase, "batch", "batchTimeout"), 0)
	setConfig(t, config.Join(traceBase, "batch", "maxQueueSize"), 0)
	setConfig(t, config.Join(traceBase, "batch", "maxExportBatchSize"), 0)
	setConfig(t, config.Join(traceBase, "timeout"), 0)
	setConfig(t, config.Join(traceBase, "retry", "enabled"), true)
	setConfig(t, config.Join(traceBase, "retry", "initialDelay"), 0)
	setConfig(t, config.Join(traceBase, "retry", "maxDelay"), 0)
	setConfig(t, config.Join(traceBase, "retry", "maxAttempts"), 0)
	setConfig(t, config.Join(traceBase, "tls", "enabled"), true)
	setConfig(t, config.Join(traceBase, "tls", "caFile"), "/definitely/missing-ca.pem")

	traceCfg := loadTraceConfig()
	if traceCfg.Batch.BatchTimeout != defaultBatchTimeout {
		t.Fatalf("BatchTimeout = %v, want %v", traceCfg.Batch.BatchTimeout, defaultBatchTimeout)
	}
	if traceCfg.Batch.MaxQueueSize != defaultMaxQueueSize {
		t.Fatalf("MaxQueueSize = %d, want %d", traceCfg.Batch.MaxQueueSize, defaultMaxQueueSize)
	}
	if traceCfg.Batch.MaxExportBatchSize != defaultMaxExportBatchSize {
		t.Fatalf(
			"MaxExportBatchSize = %d, want %d",
			traceCfg.Batch.MaxExportBatchSize,
			defaultMaxExportBatchSize,
		)
	}
	if traceCfg.Timeout != defaultTimeout {
		t.Fatalf("Timeout = %v, want %v", traceCfg.Timeout, defaultTimeout)
	}
	if traceCfg.Retry.InitialDelay != defaultRetryInitialDelay {
		t.Fatalf(
			"InitialDelay = %v, want %v",
			traceCfg.Retry.InitialDelay,
			defaultRetryInitialDelay,
		)
	}
	if traceCfg.Retry.MaxDelay != defaultRetryMaxDelay {
		t.Fatalf("MaxDelay = %v, want %v", traceCfg.Retry.MaxDelay, defaultRetryMaxDelay)
	}
	if traceCfg.Retry.MaxAttempts != defaultMaxAttempts {
		t.Fatalf("MaxAttempts = %d, want %d", traceCfg.Retry.MaxAttempts, defaultMaxAttempts)
	}

	metricBase := config.Join(config.KeyBase, "otlp", "metric")
	setConfig(t, config.Join(metricBase, "exportInterval"), 0)
	setConfig(t, config.Join(metricBase, "exportTimeout"), 0)
	setConfig(t, config.Join(metricBase, "timeout"), 0)
	setConfig(t, config.Join(metricBase, "retry", "enabled"), true)
	setConfig(t, config.Join(metricBase, "retry", "initialDelay"), 0)
	setConfig(t, config.Join(metricBase, "retry", "maxDelay"), 0)
	setConfig(t, config.Join(metricBase, "retry", "maxAttempts"), 0)
	setConfig(t, config.Join(metricBase, "tls", "enabled"), true)
	setConfig(t, config.Join(metricBase, "tls", "caFile"), "/definitely/missing-ca.pem")

	metricCfg := loadMetricConfig()
	if metricCfg.ExportInterval != defaultExportInterval {
		t.Fatalf("ExportInterval = %v, want %v", metricCfg.ExportInterval, defaultExportInterval)
	}
	if metricCfg.ExportTimeout != defaultExportTimeout {
		t.Fatalf("ExportTimeout = %v, want %v", metricCfg.ExportTimeout, defaultExportTimeout)
	}
	if metricCfg.Timeout != defaultTimeout {
		t.Fatalf("Timeout = %v, want %v", metricCfg.Timeout, defaultTimeout)
	}
	if metricCfg.Retry.InitialDelay != defaultRetryInitialDelay {
		t.Fatalf(
			"InitialDelay = %v, want %v",
			metricCfg.Retry.InitialDelay,
			defaultRetryInitialDelay,
		)
	}
	if metricCfg.Retry.MaxDelay != defaultRetryMaxDelay {
		t.Fatalf("MaxDelay = %v, want %v", metricCfg.Retry.MaxDelay, defaultRetryMaxDelay)
	}
	if metricCfg.Retry.MaxAttempts != defaultMaxAttempts {
		t.Fatalf("MaxAttempts = %d, want %d", metricCfg.Retry.MaxAttempts, defaultMaxAttempts)
	}

	if got := newGRPCTracerProvider("trace-service"); got == nil {
		t.Fatal("newGRPCTracerProvider() returned nil")
	}
	if got := newHTTPTracerProvider("trace-service"); got == nil {
		t.Fatal("newHTTPTracerProvider() returned nil")
	}
	if got := newGRPCMeterProvider("metric-service"); got == nil {
		t.Fatal("newGRPCMeterProvider() returned nil")
	}
	if got := newHTTPMeterProvider("metric-service"); got == nil {
		t.Fatal("newHTTPMeterProvider() returned nil")
	}
}
