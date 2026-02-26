# xDS Integration for Yggdrasil

## Overview

This module provides xDS-based service discovery and traffic governance for Yggdrasil.
It includes both:

1. **Resolver** (`type: xds`) for LDS/RDS/CDS/EDS driven endpoint updates.
2. **Balancer** (`balancer: xds`) for weighted selection and governance policies.

## Features

- ADS stream integration with dynamic resource subscriptions.
- Endpoint updates from xDS resources to Yggdrasil resolver state.
- Balancer policies from CDS (`round_robin`, `random`, `least_request`).
- Cluster-level governance hooks: circuit breaking, outlier detection, rate limiting.
- Example control plane and scenarios under [`example/`](./example/).

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/xds/v2
```

Enable resolver and balancer builders via side-effect import:

```go
import _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/xds/v2"
```

## Quick Start

### 1. Configure resolver and balancer

```yaml
yggdrasil:
  resolver:
    xds:
      type: xds
      config:
        name: default

  client:
    github.com.codesjoy.yggdrasil.example.sample:
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
          id: basic-client
          cluster: test-cluster
        protocol: grpc
```

### 2. Run the example control plane and sample apps

```bash
# terminal 1
cd integrations/xds/example/control-plane
go run main.go --config config.yaml

# terminal 2
cd integrations/xds/example/basic/server
go run main.go --config config.yaml

# terminal 3
cd integrations/xds/example/basic/client
go run main.go --config config.yaml
```

## Configuration

### Resolver binding (`yggdrasil.resolver.<name>.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | `string` | `default` | xDS config profile name under `yggdrasil.xds.<name>.config` |

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
| `max_retries` | `int` | `10` | ADS reconnect max retries |

### Additional parsed fields

The loader also parses `health.*` and `retry.*` keys for compatibility. Keep them if your config templates already include them.

## Examples

- Entry point: [`example/README.md`](./example/README.md)
- Control plane: [`example/control-plane`](./example/control-plane)
- Scenarios: `basic`, `canary`, `load-balancing`, `traffic-splitting`, `multi-service`

## Troubleshooting

- **Cannot connect to xDS server**: verify `server.address`, TLS files, and control plane status.
- **No endpoints in client**: ensure listener/service names and `service_map` are aligned.
- **Traffic policy not applied**: verify CDS/EDS resources include expected cluster policies and endpoint metadata.
