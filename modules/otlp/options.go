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
	"time"

	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlpmetrichttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

// createGRPCTraceClientOptions creates gRPC client options for trace exporter.
func createGRPCTraceClientOptions(cfg TraceExporterConfig) ([]otlptracegrpc.Option, error) {
	var opts []otlptracegrpc.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
	case "none", "":
		// Default (no compression)
	default:
		// Unknown compression, use default
	}

	// Configure TLS
	grpcOpts, err := createGRPCDialOptions(cfg.TLS)
	if err != nil {
		return nil, err
	}
	if len(grpcOpts) > 0 {
		opts = append(opts, otlptracegrpc.WithDialOption(grpcOpts...))
	}

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlptracegrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlptracegrpc.WithRetry(backoff))
	}

	return opts, nil
}

// createHTTPTraceClientOptions creates HTTP client options for trace exporter.
func createHTTPTraceClientOptions(cfg TraceExporterConfig) ([]otlptracehttp.Option, error) {
	var opts []otlptracehttp.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracehttp.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
	case "none", "":
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.NoCompression))
	default:
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.NoCompression))
	}

	// Configure TLS
	tlsClientOpt, err := createHTTPClientTLSOption(cfg.TLS)
	if err != nil {
		return nil, err
	}
	opts = append(opts, tlsClientOpt)

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlptracehttp.WithRetry(backoff))
	}

	return opts, nil
}

// createGRPCMeterClientOptions creates gRPC client options for metrics exporter.
func createGRPCMeterClientOptions(cfg MetricExporterConfig) ([]otlpmetricgrpc.Option, error) {
	var opts []otlpmetricgrpc.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetricgrpc.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlpmetricgrpc.WithCompressor("gzip"))
	case "none", "":
		// Default
	default:
		// Unknown compression, use default
	}

	// Configure TLS
	grpcOpts, err := createGRPCDialOptions(cfg.TLS)
	if err != nil {
		return nil, err
	}
	if len(grpcOpts) > 0 {
		opts = append(opts, otlpmetricgrpc.WithDialOption(grpcOpts...))
	}

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlpmetricgrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlpmetricgrpc.WithRetry(backoff))
	}

	return opts, nil
}

// createHTTPMeterClientOptions creates HTTP client options for metrics exporter.
func createHTTPMeterClientOptions(cfg MetricExporterConfig) ([]otlpmetrichttp.Option, error) {
	var opts []otlpmetrichttp.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetrichttp.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression))
	case "none", "":
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.NoCompression))
	default:
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.NoCompression))
	}

	// Configure TLS
	tlsClientOpt, err := createHTTPMetricClientTLSOption(cfg.TLS)
	if err != nil {
		return nil, err
	}
	opts = append(opts, tlsClientOpt)

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlpmetrichttp.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlpmetrichttp.WithRetry(backoff))
	}

	return opts, nil
}
