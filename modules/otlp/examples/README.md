# OTLP Examples

These examples run Yggdrasil v3 applications that export traces and metrics
through the OTLP module. Each example also writes JSON logs to `examples/logs`;
the local OpenTelemetry Collector reads those files with `filelog` and sends
logs to Loki.

The OTLP module itself only provides traces and metrics. The Loki path is an
example-only Collector pipeline.

## Local Observability Stack

Start the stack from the repository root:

```bash
docker compose -f modules/otlp/examples/observability/compose.yaml up -d
```

| Service | Purpose | URL or endpoint |
| --- | --- | --- |
| OpenTelemetry Collector | receives OTLP and reads example logs | `localhost:4317`, `localhost:4318` |
| Collector Prometheus exporter | metrics scrape target | `localhost:9464` |
| Prometheus | stores scraped metrics | `http://localhost:9090` |
| Loki | stores JSON logs | `http://localhost:3100` |
| Tempo | stores traces | `http://localhost:3200` |
| Grafana | Explore UI with provisioned data sources | `http://localhost:3000` |

Grafana starts with anonymous admin access and preconfigured Prometheus, Loki,
and Tempo data sources.

## Scenarios

| Scenario | Provider | Port | What it demonstrates |
| --- | --- | --- | --- |
| [quickstart](./quickstart/) | `otlp-grpc` | `8080` | default gRPC trace and metric export |
| [http-protocol](./http-protocol/) | `otlp-http` | `8081` | OTLP HTTP/protobuf export on `4318` |
| [resource-retry](./resource-retry/) | `otlp-grpc` | `8082` | resource attributes, headers, gzip, retry, short metric interval |

Run one scenario at a time from its directory so the relative config and log
paths resolve as expected. Each scenario directory contains its own README with
the exact run command, curl command, configuration focus, and Grafana queries.

## Verify Data In Grafana

Open Grafana at `http://localhost:3000`, then use Explore. The scenario README
files include service-specific queries.

Tempo queries:

- Select the `Tempo` data source.
- Search by service name: `otlp-quickstart`, `otlp-http-protocol`, or
  `otlp-resource-retry`.

Prometheus queries:

```promql
yggdrasil_otlp_http_server_requests_total
yggdrasil_otlp_http_server_request_duration_ms_bucket
```

Loki queries:

```logql
{service_name="otlp-quickstart"}
{service_name="otlp-http-protocol"}
{service_name="otlp-resource-retry"}
```

If a Loki label differs across versions, search for the log message instead:

```logql
{service_namespace="yggdrasil-examples"} |= "handled OTLP example request"
```

## How The Stack Is Wired

- Applications export traces and metrics to the Collector through OTLP gRPC
  `4317` or OTLP HTTP `4318`.
- The Collector exports traces to Tempo with OTLP gRPC.
- The Collector exposes metrics on `9464`; Prometheus scrapes that endpoint.
- Applications write JSONL logs to `examples/logs`; the Collector `filelog`
  receiver parses them and exports logs to Loki through OTLP HTTP.
- Grafana provisioning creates Prometheus, Loki, and Tempo data sources.

## Troubleshooting

Port already in use:

- Stop any local service already using `3000`, `3100`, `3200`, `4317`, `4318`,
  `9090`, or `9464`.
- Or edit `observability/compose.yaml` port mappings for local testing.

Collector cannot read logs:

- Run examples from their scenario directory.
- Confirm files appear under `modules/otlp/examples/logs`.
- Restart the Collector if it was started before the log directory existed.

Prometheus has no metrics:

- Send at least one request with `curl`.
- Wait for one scrape interval; the example configs use a short metric export
  interval and Prometheus scrapes every `5s`.
- Check `http://localhost:9464/metrics` for Collector-exported samples.

Tempo has no traces:

- Confirm the scenario uses the same protocol as its config.
- `quickstart` and `resource-retry` use `localhost:4317`; `http-protocol` uses
  `localhost:4318`.

Loki labels do not match:

- Loki may normalize resource attributes into underscore labels.
- Try `service_name`, `service_namespace`, or a text search for
  `handled OTLP example request`.

## Stop The Stack

```bash
docker compose -f modules/otlp/examples/observability/compose.yaml down
```
