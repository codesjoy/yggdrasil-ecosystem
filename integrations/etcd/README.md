# etcd Integration for Yggdrasil

## Overview

This module integrates etcd with Yggdrasil and provides three capabilities:

1. **Configuration source** (`etcd.NewConfigSource`) for remote config loading.
2. **Service registry** (`type: etcd`) backed by etcd leases.
3. **Service discovery resolver** (`type: etcd`) with watch-based updates.

## Features

- Blob and key-prefix modes for configuration loading.
- Lease-based registration with heartbeat keepalive.
- Watch-driven resolver updates with optional debounce.
- Protocol filtering for discovered endpoints.
- Ready-to-run examples under [`example/`](./example/).

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/etcd/v2
```

Register resolver/registry builders via side-effect import:

```go
import _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/etcd/v2"
```

## Quick Start

### 1. Start a local etcd instance

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

Optional health check:

```bash
etcdctl --endpoints= endpoint health
```

### 2. Configure registry and resolver

```yaml
yggdrasil:
  registry:
    type: etcd
    config:
      client:
        endpoints: [""]
        dialTimeout: 5s
      prefix: /yggdrasil/registry
      ttl: 10s
      keepAlive: true
      retryInterval: 3s

  resolver:
    default:
      type: etcd
      config:
        client:
          endpoints: [""]
          dialTimeout: 5s
        prefix: /yggdrasil/registry
        namespace: default
        protocols: [grpc, http]
        debounce: 200ms
```

### 3. (Optional) Load config from etcd

```go
import (
    "github.com/codesjoy/yggdrasil-ecosystem/integrations/etcd/v2"
    "github.com/codesjoy/yggdrasil/v2/config"
)

src, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Client: etcd.ClientConfig{Endpoints: []string{""}},
    Key:    "/demo/app/config",
})
if err != nil {
    panic(err)
}
defer src.Close()

if err := config.LoadSource(src); err != nil {
    panic(err)
}
```

## Configuration

### ClientConfig

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `client.endpoints` | `[]string` | `["127.0.0.1:2379"]` | etcd endpoints |
| `client.dialTimeout` | `duration` | `5s` | Client dial timeout |
| `client.username` | `string` | empty | Username (optional) |
| `client.password` | `string` | empty | Password (optional) |

### RegistryConfig (`yggdrasil.registry.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `prefix` | `string` | `/yggdrasil/registry` | Registry key prefix |
| `ttl` | `duration` | `10s` | Lease TTL |
| `keepAlive` | `bool` | `true` | Enable lease keepalive |
| `retryInterval` | `duration` | `3s` | Retry interval after keepalive failure |

Registry key format:

```
<prefix>/<namespace>/<service>/<instance-hash>
```

### ResolverConfig (`yggdrasil.resolver.<name>.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `prefix` | `string` | empty | Registry prefix to watch (set this explicitly) |
| `namespace` | `string` | `default` | Namespace filter |
| `protocols` | `[]string` | `[grpc,http]` | Allowed endpoint protocols |
| `debounce` | `duration` | `0` | Debounce window for watch events |

### ConfigSourceConfig (`etcd.NewConfigSource`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `key` | `string` | empty | Blob mode key |
| `prefix` | `string` | empty | KV mode key prefix |
| `mode` | `string` | inferred | `blob` or `kv` |
| `watch` | `bool` | `true` | Enable config watch |
| `format` | `parser` | YAML parser | Value parser |
| `name` | `string` | inferred | Source name |

Rules:

- Exactly one of `key` or `prefix` must be set.
- Mode is inferred automatically (`key -> blob`, `prefix -> kv`).

## Examples

- [`example/allinone`](./example/allinone): config source + registry + resolver together.
- [`example/config-source/blob`](./example/config-source/blob): single-key config.
- [`example/config-source/kv`](./example/config-source/kv): prefix-based hierarchical config.
- [`example/registry`](./example/registry): lease-based service registration.
- [`example/resolver`](./example/resolver): watch and print discovered endpoints.

## Troubleshooting

- **Cannot connect to etcd**: verify endpoint/credentials and run `etcdctl endpoint health`.
- **Resolver returns empty endpoints**: check `prefix`, `namespace`, and `protocols` filters.
- **Config source has no updates**: confirm `watch` is enabled and keys are updated under the exact `key`/`prefix`.
