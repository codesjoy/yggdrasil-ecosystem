# OTLP HTTP Protocol Example

This example uses the `otlp-http` tracer and meter providers. It exports traces
and metrics to the local OpenTelemetry Collector OTLP HTTP receiver on
`localhost:4318` and listens for HTTP requests on `localhost:8081`.

## Configuration Focus

The example config uses:

- `tracer: otlp-http`
- `meter: otlp-http`
- trace and metric endpoint `localhost:4318`
- insecure local transport for the Collector
- resource attribute `example.protocol: http`

## Run

Start the local observability stack from the repository root:

```bash
docker compose -f modules/otlp/examples/observability/compose.yaml up -d
```

Run the example:

```bash
cd modules/otlp/examples/http-protocol
go run .
```

Generate telemetry from another terminal:

```bash
curl http://localhost:8081
```

## Grafana Queries

Open Grafana at `http://localhost:3000` and use Explore.

Tempo:

- Select `Tempo`.
- Search for service `otlp-http-protocol`.

Prometheus:

```promql
yggdrasil_otlp_http_server_requests_total{service_name="otlp-http-protocol"}
yggdrasil_otlp_http_server_request_duration_ms_bucket{service_name="otlp-http-protocol"}
```

Loki:

```logql
{service_name="otlp-http-protocol"}
```

If labels differ, search for the message:

```logql
{} |= "handled OTLP example request" |= "otlp-http-protocol"
```
