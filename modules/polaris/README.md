# Polaris Module for Yggdrasil v3

This module integrates Polaris with Yggdrasil v3 through the module/capability
runtime. It provides:

- `polaris.Module()` for registry, resolver, balancer, and client governance
  interceptor capabilities.
- `polaris.WithModule()` as a convenience Yggdrasil option.
- `kind: polaris` declarative config sources under `yggdrasil.config.sources`.

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3
```

Register the module explicitly:

```go
app, err := yggdrasil.New(
    yggdrasil.WithAppName("example"),
    polaris.WithModule(),
)
```

Blank-import side-effect registration is not supported in v3.

See [`examples/quickstart`](./examples/quickstart) for a runnable server/client
example backed by a local Polaris standalone server.

## Configuration

Only the Yggdrasil v3 configuration shape is supported.

```yaml
yggdrasil:
  polaris:
    sdks:
      default:
        addresses:
          - "127.0.0.1:8091"
        config_addresses:
          - "127.0.0.1:8093"
        token: ""
        config_file: ""
    governance:
      defaults:
        namespace: default
        caller_service: example-client
        routing:
          enable: true
          recover_all: true
        rate_limit:
          enable: false
        circuit_breaker:
          enable: false

  discovery:
    registry:
      type: polaris
      config:
        sdk: default
        namespace: default
        service_token: ""
        ttl: 5s
        auto_heartbeat: true
        timeout: 2s
        retry_count: 0
    resolvers:
      polaris:
        type: polaris
        config:
          sdk: default
          namespace: default
          protocols: [grpc]
          refresh_interval: 10s
          timeout: 2s
          retry_count: 0
          skip_route_filter: true

  balancers:
    defaults:
      polaris:
        type: polaris

  clients:
    services:
      example-server:
        resolver: polaris
        balancer: polaris
```

## Config Source

`polaris.WithModule()` registers a declarative source builder. Keep Polaris SDK
connection settings in the local bootstrap config, then declare the remote file
under `yggdrasil.config.sources`:

```yaml
yggdrasil:
  polaris:
    sdks:
      default:
        addresses: ["127.0.0.1:8091"]
        config_addresses: ["127.0.0.1:8093"]

  config:
    sources:
      - kind: polaris
        name: polaris:app
        priority: remote
        config:
          sdk: default
          namespace: default
          file_group: app
          file_name: service.yaml
          fetch_timeout: 2s
```

The config source implements the v3 `config/source.Source` and
`config/source.Watchable` contracts. `polaris.WithConfigSource(...)` remains
available for advanced programmatic bootstraps.
