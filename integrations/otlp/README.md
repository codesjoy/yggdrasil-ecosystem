# OTLP Exporter Integration for Yggdrasil

## Overview

This module adds OTLP exporters for traces and metrics in Yggdrasil.
It supports both gRPC and HTTP exporters and can be enabled through Yggdrasil config.

## Features

- OTLP trace exporter (`otlp-grpc`, `otlp-http`).
- OTLP metric exporter (`otlp-grpc`, `otlp-http`).
- Configurable TLS, headers, timeout, compression, and retry.
- Configurable trace batch options and metric export interval.
- Fail-safe behavior: exporter init errors fall back to noop providers.

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/otlp/v2
```

Enable exporter builders via side-effect import:

```go
import _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/otlp/v2"
```

## Quick Start

### 1. Configure tracer and meter

```yaml
yggdrasil:
  tracer: otlp-grpc
  meter: otlp-grpc

  otlp:
    trace:
      endpoint: localhost:4317
      tls:
        insecure: true
    metric:
      endpoint: localhost:4317
      tls:
        insecure: true
```

### 2. Initialize your app

```go
import (
    _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/otlp/v2"
    "github.com/codesjoy/yggdrasil/v2"
)

func main() {
    yggdrasil.Init("my-service")
    yggdrasil.Run()
}
```

### 3. Run a local OTLP backend (Jaeger all-in-one)

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

## Configuration

### Trace (`yggdrasil.otlp.trace`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `endpoint` | `string` | `localhost:4317` or `localhost:4318` | OTLP endpoint (depends on exporter type) |
| `protocol` | `string` | exporter-defined | `grpc` or `http` (mainly for direct API usage) |
| `tls.enabled` | `bool` | `false` | Enable TLS |
| `tls.insecure` | `bool` | `false` | Skip TLS verification |
| `tls.caFile` | `string` | empty | CA certificate path |
| `tls.certFile` | `string` | empty | Client cert path |
| `tls.keyFile` | `string` | empty | Client key path |
| `headers` | `map[string]string` | empty | Extra request headers |
| `timeout` | `duration` | `30s` | Export timeout |
| `compression` | `string` | none | `gzip` or `none` |
| `retry.enabled` | `bool` | `false` | Enable retry |
| `retry.maxAttempts` | `int` | `5` (if retry enabled) | Retry attempts |
| `retry.initialDelay` | `duration` | `100ms` | Retry initial interval |
| `retry.maxDelay` | `duration` | `5s` | Retry max interval |
| `batch.batchTimeout` | `duration` | `5s` | Batch timeout |
| `batch.maxQueueSize` | `int` | `2048` | Queue size |
| `batch.maxExportBatchSize` | `int` | `512` | Batch size |
| `resource` | `map[string]any` | empty | Resource attributes |

### Metric (`yggdrasil.otlp.metric`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `endpoint` | `string` | `localhost:4317` or `localhost:4318` | OTLP endpoint |
| `protocol` | `string` | exporter-defined | `grpc` or `http` |
| `tls.*` | - | same as trace | TLS options |
| `headers` | `map[string]string` | empty | Extra request headers |
| `timeout` | `duration` | `30s` | Export timeout |
| `compression` | `string` | none | `gzip` or `none` |
| `retry.*` | - | same as trace | Retry options |
| `exportInterval` | `duration` | `60s` | Metric export interval |
| `exportTimeout` | `duration` | `30s` | Metric export timeout |
| `temporality` | `string` | `cumulative` | Kept for config compatibility |
| `resource` | `map[string]any` | empty | Resource attributes |

Notes:

- `otlp-grpc` and `otlp-http` exporter names decide the protocol at runtime.
- If TLS is not enabled, the exporter uses insecure transport by default (development-friendly).

## Examples

- Quick guide: [`QUICKSTART.md`](./QUICKSTART.md)
- Runnable sample: [`example/`](./example/)

## Troubleshooting

- **No traces/metrics**: verify backend endpoint and exporter type (`otlp-grpc` vs `otlp-http`).
- **Connection refused**: check backend port (`4317` for gRPC, `4318` for HTTP).
- **TLS handshake errors**: verify `tls.enabled`, certificates, and CA chain.
