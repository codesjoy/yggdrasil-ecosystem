# OTLP gRPC Quickstart

This example uses the `otlp-grpc` tracer and meter providers. It exports traces
and metrics to the local OpenTelemetry Collector on `localhost:4317` and listens
for HTTP requests on `localhost:8080`.

## Configuration Focus

The example config uses:

- `tracer: otlp-grpc`
- `meter: otlp-grpc`
- trace and metric endpoint `localhost:4317`
- insecure local transport for the Collector
- short metric export interval for quick local feedback

## Run

Start the local observability stack from the repository root:

```bash
docker compose -f modules/otlp/examples/observability/compose.yaml up -d
```

Run the example:

```bash
cd modules/otlp/examples/quickstart
go run .
```

Generate telemetry from another terminal:

```bash
curl http://localhost:8080
```

The application writes JSON logs to `modules/otlp/examples/logs`; the Collector
reads that directory and forwards logs to Loki.

## Grafana Queries

Open Grafana at `http://localhost:3000` and use Explore.

Tempo:

- Select `Tempo`.
- Search for service `otlp-quickstart`.

Prometheus:

```promql
yggdrasil_otlp_http_server_requests_total{service_name="otlp-quickstart"}
yggdrasil_otlp_http_server_request_duration_ms_bucket{service_name="otlp-quickstart"}
```

Loki:

```logql
{service_name="otlp-quickstart"}
```

If labels differ, search for the message:

```logql
{} |= "handled OTLP example request" |= "otlp-quickstart"
```
