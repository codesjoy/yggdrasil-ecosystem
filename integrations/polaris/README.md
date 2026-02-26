# Polaris Integration for Yggdrasil

## Overview

This module integrates Polaris with Yggdrasil and covers:

1. **Service registry** (`type: polaris`).
2. **Service resolver** (`type: polaris`).
3. **Config source** (`polaris.NewConfigSource`).
4. **Governance** via Polaris balancer and client interceptors.

## Features

- SDKContext reuse by SDK name (`yggdrasil.polaris.<name>`).
- Instance registration/deregistration with metadata and location support.
- Resolver polling with protocol filtering.
- Polaris config center source with optional subscribe mode.
- Governance hooks for routing, rate limiting, and circuit breaking.

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/polaris/v2
```

Enable builders/interceptors via side-effect import:

```go
import _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/polaris/v2"
```

## Quick Start

### 1. Configure Polaris SDK context

```yaml
yggdrasil:
  polaris:
    default:
      addresses:
        - "127.0.0.1:8091"
      config_addresses:
        - "127.0.0.1:8093"
      token: ""
      config_file: ""
```

### 2. Enable registry on server side

```yaml
yggdrasil:
  registry:
    type: polaris
    config:
      sdk: default
      namespace: default
      serviceToken: ""
      ttl: 5s
      autoHeartbeat: true
      timeout: 2s
      retryCount: 0
```

### 3. Enable resolver + polaris balancer on client side

```yaml
yggdrasil:
  balancer:
    polaris:
      type: polaris

  resolver:
    default:
      type: polaris
      config:
        sdk: default
        namespace: default
        protocols: [grpc]
        refreshInterval: 10s
        timeout: 2s
        retryCount: 0
        skipRouteFilter: true

  client:
    example.hello.server:
      resolver: default
      balancer: polaris
```

### 4. (Optional) Load config from Polaris config center

```go
import (
    "github.com/codesjoy/yggdrasil-ecosystem/integrations/polaris/v2"
    "github.com/codesjoy/yggdrasil/v2/config"
)

src, err := polaris.NewConfigSource(polaris.ConfigSourceConfig{
    SDK:       "default",
    Namespace: "default",
    FileGroup: "app",
    FileName:  "service.yaml",
})
if err != nil {
    panic(err)
}
if err := config.LoadSource(src); err != nil {
    panic(err)
}
```

## Configuration

### SDKConfig (`yggdrasil.polaris.<sdkName>`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `addresses` | `[]string` | empty | Polaris naming addresses |
| `config_addresses` | `[]string` | empty | Polaris config-center addresses |
| `token` | `string` | empty | Polaris token |
| `config_file` | `string` | empty | Polaris native config file |

Notes:

- If `config_file` is set, file-based SDK initialization is preferred.
- `config_addresses` falls back to `addresses` when not provided.

### RegistryConfig (`yggdrasil.registry.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `sdk` | `string` | `default` | SDK context name |
| `namespace` | `string` | instance namespace or `default` | Target namespace |
| `serviceToken` | `string` | empty | Service token |
| `ttl` | `duration` | `0` | Instance TTL |
| `autoHeartbeat` | `bool` | `false` | Enable SDK heartbeat |
| `timeout` | `duration` | `0` | Request timeout |
| `retryCount` | `int` | `0` | Retry count |

### ResolverConfig (`yggdrasil.resolver.<name>.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `sdk` | `string` | resolver name | SDK context name |
| `namespace` | `string` | `default` | Target namespace |
| `protocols` | `[]string` | `[grpc]` | Allowed endpoint protocols |
| `refreshInterval` | `duration` | `10s` | Poll interval |
| `timeout` | `duration` | `0` | Request timeout |
| `retryCount` | `int` | `0` | Retry count |
| `skipRouteFilter` | `bool` | `false` | Skip Polaris route filtering in resolver |
| `metadata` | `map[string]string` | nil | Request metadata |

### ConfigSourceConfig (`polaris.NewConfigSource`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `sdk` | `string` | `default` | SDK context name |
| `namespace` | `string` | `yggdrasil.InstanceNamespace()` | Config namespace |
| `fileGroup` | `string` | `default` | Config file group |
| `fileName` | `string` | required | Config file name |
| `subscribe` | `bool` | `true` | Enable change subscription |
| `mode` | `int` | SDK default mode | Polaris fetch mode |
| `format` | `parser` | inferred by extension | Parse format |
| `fetchTimeout` | `duration` | no timeout | Fetch timeout |

### Governance config (`yggdrasil.polaris.governance`)

Common path: `yggdrasil.polaris.governance.config`.

- `routing.enable`: enable Polaris routing.
- `rateLimit.enable`: enable Polaris rate limiting.
- `circuitBreaker.enable`: enable Polaris circuit breaking.

Per-service overrides are supported via `yggdrasil.polaris.governance.{<serviceName>}.config`.

## Examples

- Entry point: [`example/`](./example/)
- Sample server/client: [`example/sample`](./example/sample)
- Scenarios: config source, governance, multi-sdk, instance metadata under [`example/scenarios`](./example/scenarios)

## Troubleshooting

- **No instances discovered**: verify namespace, service name, and `protocols` filter.
- **Registry succeeds but no traffic**: ensure client uses `resolver: <polaris-resolver>` and `balancer: polaris`.
- **Config source not updating**: check `subscribe: true`, `fileGroup`, and `fileName`.
