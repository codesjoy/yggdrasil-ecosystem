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

	httpTraceOpts, err := createHTTPTraceClientOptions(traceCfg)
	if err != nil {
		t.Fatalf("createHTTPTraceClientOptions() error = %v", err)
	}
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

	httpMetricOpts, err := createHTTPMeterClientOptions(metricCfg)
	if err != nil {
		t.Fatalf("createHTTPMeterClientOptions() error = %v", err)
	}
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
			opts, err := createGRPCDialOptions(tt.cfg)
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
	if opt, err := createHTTPClientTLSOption(TLSConfig{Insecure: true}); err != nil || opt == nil {
		t.Fatal("createHTTPClientTLSOption() returned nil for insecure config")
	}
	if opt, err := createHTTPClientTLSOption(TLSConfig{Enabled: true}); err != nil || opt == nil {
		t.Fatal("createHTTPClientTLSOption() returned nil for TLS config")
	}
	if _, err := createHTTPClientTLSOption(TLSConfig{
		Enabled: true,
		CAFile:  "/definitely/missing-ca.pem",
	}); err == nil {
		t.Fatal("createHTTPClientTLSOption() expected error for missing CA file")
	}
	if opt, err := createHTTPMetricClientTLSOption(TLSConfig{Insecure: true}); err != nil ||
		opt == nil {
		t.Fatal("createHTTPMetricClientTLSOption() returned nil for insecure config")
	}
	if opt, err := createHTTPMetricClientTLSOption(TLSConfig{Enabled: true}); err != nil ||
		opt == nil {
		t.Fatal("createHTTPMetricClientTLSOption() returned nil for TLS config")
	}
	if _, err := createHTTPMetricClientTLSOption(TLSConfig{
		Enabled: true,
		CAFile:  "/definitely/missing-ca.pem",
	}); err == nil {
		t.Fatal("createHTTPMetricClientTLSOption() expected error for missing CA file")
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
	traceCfg := applyTraceDefaults(TraceExporterConfig{
		Retry: RetryConfig{Enabled: true},
		TLS:   TLSConfig{Enabled: true, CAFile: "/definitely/missing-ca.pem"},
	})
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

	metricCfg := applyMetricDefaults(MetricExporterConfig{
		Retry: RetryConfig{Enabled: true},
		TLS:   TLSConfig{Enabled: true, CAFile: "/definitely/missing-ca.pem"},
	})
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

	mod := &otlpModule{
		settings: Config{
			Trace:  traceCfg,
			Metric: metricCfg,
		},
	}
	if got := mod.newGRPCTracerProvider("trace-service"); got == nil {
		t.Fatal("newGRPCTracerProvider() returned nil")
	}
	if got := mod.newHTTPTracerProvider("trace-service"); got == nil {
		t.Fatal("newHTTPTracerProvider() returned nil")
	}
	if got := mod.newGRPCMeterProvider("metric-service"); got == nil {
		t.Fatal("newGRPCMeterProvider() returned nil")
	}
	if got := mod.newHTTPMeterProvider("metric-service"); got == nil {
		t.Fatal("newHTTPMeterProvider() returned nil")
	}
}
