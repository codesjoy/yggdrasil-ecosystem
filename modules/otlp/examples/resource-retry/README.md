# OTLP Resource And Retry Example

This example uses the `otlp-grpc` tracer and meter providers with extra
exporter options. It exports to the local OpenTelemetry Collector on
`localhost:4317` and listens for HTTP requests on `localhost:8082`.

## Configuration Focus

The example config demonstrates:

- resource attributes such as `service.namespace` and `example.scenario`
- custom OTLP headers
- gzip compression
- retry settings
- short metric export interval
- cumulative temporality compatibility field

The OTLP module keeps `temporality` as a config-compatible field; it does not
install metric temporality views.

## Run

Start the local observability stack from the repository root:

```bash
docker compose -f modules/otlp/examples/observability/compose.yaml up -d
```

Run the example:

```bash
cd modules/otlp/examples/resource-retry
go run .
```

Generate telemetry from another terminal:

```bash
curl http://localhost:8082
```

## Grafana Queries

Open Grafana at `http://localhost:3000` and use Explore.

Tempo:

- Select `Tempo`.
- Search for service `otlp-resource-retry`.

Prometheus:

```promql
yggdrasil_otlp_http_server_requests_total{service_name="otlp-resource-retry"}
yggdrasil_otlp_http_server_request_duration_ms_bucket{service_name="otlp-resource-retry"}
```

Loki:

```logql
{service_name="otlp-resource-retry"}
```

If labels differ, search by namespace or message:

```logql
{service_namespace="yggdrasil-examples"} |= "resource-retry"
{} |= "handled OTLP example request" |= "otlp-resource-retry"
```
