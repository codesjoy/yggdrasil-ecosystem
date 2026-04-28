# OTLP Module for Yggdrasil v3

This module provides OpenTelemetry OTLP trace and metric providers for
Yggdrasil v3. It supports gRPC and HTTP exporters and is registered explicitly
through the v3 module system.

It does not provide an OTLP logs provider. The examples show logs through a
local Collector `filelog` pipeline so application runtime dependencies stay
limited to traces and metrics.

## Providers

The module registers four named providers:

| Provider | Capability | Protocol | Default endpoint |
| --- | --- | --- | --- |
| `otlp-grpc` | tracer | gRPC | `localhost:4317` |
| `otlp-http` | tracer | HTTP/protobuf | `localhost:4318` |
| `otlp-grpc` | meter | gRPC | `localhost:4317` |
| `otlp-http` | meter | HTTP/protobuf | `localhost:4318` |

Use `otlp-grpc` when the Collector exposes the standard OTLP gRPC receiver on
`4317`. Use `otlp-http` when the Collector exposes the standard OTLP HTTP
receiver on `4318`.

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/otlp/v3
```

## Usage

Register the module explicitly:

```go
import (
	"context"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/otlp/v3"
	"github.com/codesjoy/yggdrasil/v3"
)

func main() {
	_ = yggdrasil.Run(
		context.Background(),
		"my-service",
		compose,
		yggdrasil.WithConfigPath("config.yaml"),
		otlp.WithModule(),
	)
}
```

Configure provider names and OTLP settings under
`yggdrasil.observability.telemetry.providers.otlp`:

```yaml
yggdrasil:
  observability:
    telemetry:
      tracer: otlp-grpc
      meter: otlp-grpc
      providers:
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

Blank-import side-effect registration is not supported in v3.

## Configuration

Trace config lives at
`yggdrasil.observability.telemetry.providers.otlp.trace`.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `endpoint` | `string` | provider endpoint | OTLP endpoint without scheme, for example `localhost:4317` |
| `protocol` | `string` | provider-defined | `grpc` or `http`; mainly for direct `NewTracerProvider` usage |
| `tls.enabled` | `bool` | `false` | Enable TLS transport |
| `tls.insecure` | `bool` | `false` | Use insecure transport; examples use this for local Collector |
| `tls.caFile` | `string` | empty | CA certificate path |
| `tls.certFile` | `string` | empty | Client certificate path |
| `tls.keyFile` | `string` | empty | Client key path |
| `headers` | `map[string]string` | empty | Extra exporter request headers |
| `timeout` | `duration` | `30s` | Export request timeout |
| `compression` | `string` | none | `gzip` or `none` |
| `retry.enabled` | `bool` | `false` | Enable exporter retry |
| `retry.maxAttempts` | `int` | `5` when retry is enabled | Retry attempts used to derive max elapsed time |
| `retry.initialDelay` | `duration` | `100ms` | Retry initial interval |
| `retry.maxDelay` | `duration` | `5s` | Retry max interval |
| `batch.batchTimeout` | `duration` | `5s` | Trace batch timeout |
| `batch.maxQueueSize` | `int` | `2048` | Trace batch queue size |
| `batch.maxExportBatchSize` | `int` | `512` | Trace export batch size |
| `resource` | `map[string]any` | empty | Resource attributes merged with `service.name` |

Metric config lives at
`yggdrasil.observability.telemetry.providers.otlp.metric`.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `endpoint` | `string` | provider endpoint | OTLP endpoint without scheme, for example `localhost:4318` |
| `protocol` | `string` | provider-defined | `grpc` or `http`; mainly for direct `NewMeterProvider` usage |
| `tls.*` | - | same as trace | TLS options |
| `headers` | `map[string]string` | empty | Extra exporter request headers |
| `timeout` | `duration` | `30s` | Export request timeout |
| `compression` | `string` | none | `gzip` or `none` |
| `retry.*` | - | same as trace | Retry options |
| `exportInterval` | `duration` | `60s` | Periodic metric export interval |
| `exportTimeout` | `duration` | `30s` | Periodic metric export timeout |
| `temporality` | `string` | `cumulative` | Compatibility field; current provider does not install temporality views |
| `resource` | `map[string]any` | empty | Resource attributes merged with `service.name` |

TLS certificate and key files are loaded when TLS is enabled. Missing or invalid
files cause provider creation to fail; module capability builders log the error
and fall back to noop providers.

## Examples

Runnable examples and local observability stack: [examples/](./examples/)
