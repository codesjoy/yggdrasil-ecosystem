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

package exampleapp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/otlp/v3"
	"github.com/codesjoy/yggdrasil/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Settings describes one runnable OTLP example scenario.
type Settings struct {
	Name       string
	ConfigPath string
	ListenAddr string
}

// Run starts an HTTP workload that emits traces, metrics, and JSON logs.
func Run(settings Settings) error {
	if settings.Name == "" {
		settings.Name = "otlp-example"
	}
	if settings.ConfigPath == "" {
		settings.ConfigPath = "config.yaml"
	}
	if settings.ListenAddr == "" {
		settings.ListenAddr = ":8080"
	}

	logFile, err := configureLogger(settings.Name)
	if err != nil {
		return err
	}
	defer logFile.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return yggdrasil.Run(
		ctx,
		settings.Name,
		compose(settings),
		yggdrasil.WithConfigPath(settings.ConfigPath),
		otlp.WithModule(),
	)
}

type httpTask struct {
	name      string
	addr      string
	server    *http.Server
	tracer    trace.Tracer
	counter   metric.Int64Counter
	histogram metric.Float64Histogram
}

func compose(settings Settings) func(yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
	return func(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
		meter := rt.MeterProvider().Meter(settings.Name)
		requestCounter, err := meter.Int64Counter(
			"http_server_requests_total",
			metric.WithDescription("Total number of HTTP requests handled by the OTLP example"),
			metric.WithUnit("1"),
		)
		if err != nil {
			return nil, err
		}

		requestDuration, err := meter.Float64Histogram(
			"http_server_request_duration_ms",
			metric.WithDescription("HTTP request duration in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			return nil, err
		}

		task := &httpTask{
			name:      settings.Name,
			addr:      settings.ListenAddr,
			tracer:    rt.TracerProvider().Tracer(settings.Name),
			counter:   requestCounter,
			histogram: requestDuration,
		}
		task.server = &http.Server{
			Addr:              settings.ListenAddr,
			Handler:           http.HandlerFunc(task.handleRequest),
			ReadHeaderTimeout: 15 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}

		return &yggdrasil.BusinessBundle{
			Tasks: []yggdrasil.BackgroundTask{task},
		}, nil
	}
}

func (t *httpTask) Serve() error {
	slog.Info("OTLP example server started",
		slog.String("listen_addr", t.addr),
		slog.String("service.name", t.name),
	)
	slog.Info("generate telemetry with curl",
		slog.String("url", "http://localhost"+t.addr),
	)

	if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (t *httpTask) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return t.server.Shutdown(ctx)
}

func (t *httpTask) handleRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, span := t.tracer.Start(r.Context(), "example.http.request")
	defer span.End()

	status := http.StatusOK
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", r.Method),
		attribute.String("url.path", r.URL.Path),
		attribute.String("server.address", r.Host),
		attribute.String("service.name", t.name),
	}
	span.SetAttributes(attrs...)

	workDuration := t.simulateWork(ctx)
	durationMillis := float64(time.Since(start).Microseconds()) / 1000

	t.counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.Int("status", status),
			attribute.String("service.name", t.name),
		),
	)
	t.histogram.Record(ctx, durationMillis,
		metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.Int("status", status),
			attribute.String("service.name", t.name),
		),
	)

	span.SetAttributes(
		attribute.Int("http.response.status_code", status),
		attribute.Float64("example.duration_ms", durationMillis),
	)

	logAttrs := []any{
		slog.String("service.name", t.name),
		slog.String("http.method", r.Method),
		slog.String("url.path", r.URL.Path),
		slog.Int("http.status_code", status),
		slog.Float64("duration_ms", durationMillis),
		slog.Duration("work_duration", workDuration),
	}
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		logAttrs = append(logAttrs,
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	slog.InfoContext(ctx, "handled OTLP example request", logAttrs...)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "service: %s\n", t.name)
	_, _ = fmt.Fprintf(w, "trace, metric, and log events emitted\n")
	_, _ = fmt.Fprintf(w, "duration_ms: %.2f\n", durationMillis)
}

func (t *httpTask) simulateWork(ctx context.Context) time.Duration {
	ctx, span := t.tracer.Start(ctx, "example.simulated_work")
	defer span.End()

	delay := time.Duration(25+rand.Intn(75)) * time.Millisecond // nolint:gosec
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		span.RecordError(ctx.Err())
	case <-timer.C:
		span.AddEvent("simulated work completed")
	}
	span.SetAttributes(attribute.Int64("example.work_ms", delay.Milliseconds()))
	return delay
}

func configureLogger(serviceName string) (*os.File, error) {
	logDir := filepath.Join("..", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, serviceName+".jsonl")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	writer := io.MultiWriter(os.Stdout, logFile)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With(
		slog.String("service.name", serviceName),
		slog.String("deployment.environment", "local"),
	)
	slog.SetDefault(logger)
	return logFile, nil
}
