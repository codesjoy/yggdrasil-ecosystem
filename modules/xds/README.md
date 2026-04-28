# xDS Integration for Yggdrasil

## Overview

This module provides xDS-based service discovery and traffic governance for Yggdrasil v3.
It includes both:

1. **Resolver** (`type: xds`) for LDS/RDS/CDS/EDS driven endpoint updates.
2. **Balancer** (`balancer: xds`) for weighted selection and governance policies.

## Features

- ADS stream integration with dynamic resource subscriptions.
- Endpoint updates from xDS resources to Yggdrasil resolver state.
- Balancer policies from CDS (`round_robin`, `random`, `least_request`).
- Cluster-level governance hooks: circuit breaking, outlier detection, rate limiting.
- Example control plane and scenarios under [`examples/`](./examples/).

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3
```

Register the module with your v3 app:

```go
import "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3"

app, err := yapp.New(
    "example.client",
    yapp.WithModules(xds.Module()),
)
```

## Quick Start

### 1. Configure resolver and balancer

```yaml
yggdrasil:
  discovery:
    resolvers:
      xds:
        type: xds
        config:
          name: default

  balancers:
    defaults:
      xds:
        type: xds
        config: {}

  clients:
    services:
      sample:
        resolver: xds
        balancer: xds

  xds:
    default:
      config:
        server:
          address: 127.0.0.1:18000
          timeout: 5s
          tls:
            enable: false
        node:
          id: quickstart-client
          cluster: xds-examples
        protocol: grpc
```

Programmatic registration is also available:

```go
err := yggdrasil.Run(
    context.Background(),
    "example.server",
    compose,
    yggdrasil.WithConfigPath("config.yaml"),
    xds.WithModule(),
)
```

## Package Layout

The root package is intentionally small:

- `xds.WithModule()` / `xds.Module()` register resolver and balancer capabilities.

Public subpackages expose the advanced APIs:

- `discovery` contains the public resolver config types plus `NewResolver()` and
  `ResolverProvider()`.
- `traffic` contains the balancer provider and governance runtime types.

Internal implementation is split by responsibility:

- `internal/resolver` contains ADS connection management, subscription syncing,
  resolver runtime state, and config decoding.
- `internal/resource` contains shared xDS resource models, protobuf parsing, and
  route matching used by resolver and balancer internals.

### 2. Run the shared control plane and quickstart apps

```bash
# terminal 1
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./quickstart/xds/snapshot.yaml

# terminal 2
cd modules/xds/examples/quickstart/server
go run .

# terminal 3
cd modules/xds/examples/quickstart/client
go run .
```

## Configuration

### Resolver binding (`yggdrasil.discovery.resolvers.<name>.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | `string` | `default` | xDS config profile name under `yggdrasil.xds.<name>.config` |

### Balancer binding (`yggdrasil.balancers.defaults.xds`)

```yaml
yggdrasil:
  balancers:
    defaults:
      xds:
        type: xds
        config: {}
```

### xDS profile (`yggdrasil.xds.<profile>.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `server.address` | `string` | `127.0.0.1:18000` | ADS server address |
| `server.timeout` | `duration` | `5s` | Dial timeout |
| `server.tls.enable` | `bool` | `false` | Enable TLS |
| `server.tls.cert_file` | `string` | empty | Client cert file |
| `server.tls.key_file` | `string` | empty | Client key file |
| `server.tls.ca_file` | `string` | empty | CA file |
| `node.id` | `string` | `yggdrasil-node` | Node ID |
| `node.cluster` | `string` | `yggdrasil-cluster` | Node cluster |
| `node.metadata` | `map[string]string` | empty | Node metadata |
| `node.locality.region` | `string` | empty | Locality region |
| `node.locality.zone` | `string` | empty | Locality zone |
| `node.locality.sub_zone` | `string` | empty | Locality sub-zone |
| `protocol` | `string` | `grpc` | Endpoint protocol label |
| `service_map` | `map[string]string` | empty | App name to listener mapping |
| `max_retries` | `int` | `0` | ADS reconnect max retries; `0` means unlimited reconnects |

### Additional parsed fields

The loader also parses `health.*` and `retry.*` keys for compatibility. Keep them if your config templates already include them.

## Examples

- Entry point: [`examples/README.md`](./examples/README.md)
- Shared control plane command: [`examples/cmd/control-plane`](./examples/cmd/control-plane)
- Scenarios: `quickstart`, `load-balancing`, `canary`, `traffic-splitting`, `multi-service`

## Troubleshooting

- **Cannot connect to xDS server**: verify `server.address`, TLS files, and control plane status.
- **No endpoints in client**: ensure the client target matches the listener name, or configure `service_map` when you intentionally use different names.
- **Traffic policy not applied**: verify CDS/EDS resources include expected cluster policies and endpoint metadata.
