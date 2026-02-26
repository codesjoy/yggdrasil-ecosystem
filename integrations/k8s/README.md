# Kubernetes Integration for Yggdrasil

## Overview

This module provides Kubernetes-native integration for Yggdrasil:

1. **Service discovery resolver** (`type: kubernetes`).
2. **Configuration sources** from ConfigMap and Secret.

## Features

- EndpointSlice-first discovery with Endpoints fallback.
- Resolver updates mapped to Yggdrasil `resolver.State`.
- Config loading from ConfigMap/Secret with optional watch updates.
- File-format inference for config keys (`.yaml`, `.yml`, `.json`, `.toml`).
- Minimal runnable examples under [`example/`](./example/).

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/k8s/v2
```

Enable resolver builder via side-effect import:

```go
import _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/k8s/v2"
```

## Quick Start

### 1. Configure Kubernetes resolver

```yaml
yggdrasil:
  resolver:
    my-k8s:
      type: kubernetes
      config:
        namespace: default
        mode: endpointslice
        portName: grpc
        protocol: grpc
        kubeconfig: "" # empty => in-cluster config
        backoff:
          baseDelay: 1s
          multiplier: 1.6
          jitter: 0.2
          maxDelay: 30s

  client:
    my-service:
      resolver: my-k8s
      balancer: default
```

### 2. Load configuration from ConfigMap (optional)

```go
import (
    k8s "github.com/codesjoy/yggdrasil-ecosystem/integrations/k8s/v2"
    "github.com/codesjoy/yggdrasil/v2/config"
    "github.com/codesjoy/yggdrasil/v2/config/source"
)

src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Watch:     true,
    Priority:  source.PriorityRemote,
})
if err != nil {
    panic(err)
}
if err := config.LoadSource(src); err != nil {
    panic(err)
}
```

Use `k8s.NewSecretSource(...)` the same way for Secret-backed config.

### 3. RBAC minimum permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: yggdrasil-k8s
  namespace: default
rules:
  - apiGroups: [""]
    resources: ["endpoints", "endpointslices", "configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
```

## Configuration

### ResolverConfig (`yggdrasil.resolver.<name>.config`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `namespace` | `string` | `KUBERNETES_NAMESPACE` or `default` | Namespace to watch |
| `mode` | `string` | `endpointslice` | `endpointslice` or `endpoints` |
| `portName` | `string` | empty | Preferred port name |
| `port` | `int32` | `0` | Fallback port number |
| `protocol` | `string` | `grpc` | Endpoint protocol label |
| `kubeconfig` | `string` | empty | Local kubeconfig path |
| `endpointAttributes` | `map[string]string` | nil | Extra endpoint attributes |
| `backoff.baseDelay` | `duration` | `1s` | Initial reconnect delay |
| `backoff.multiplier` | `float64` | `1.6` | Backoff multiplier |
| `backoff.jitter` | `float64` | `0.2` | Backoff jitter |
| `backoff.maxDelay` | `duration` | `30s` | Max reconnect delay |
| `resyncPeriod` | `duration` | `0` | Reserved field |
| `timeout` | `duration` | `0` | Reserved field |

Notes:

- `portName` has higher priority than `port`.
- If neither `portName` nor `port` is set, the first endpoint port is used.

### ConfigSourceConfig (`NewConfigMapSource` / `NewSecretSource`)

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `namespace` | `string` | empty | Resource namespace (set explicitly) |
| `name` | `string` | required | ConfigMap/Secret name |
| `key` | `string` | inferred | Key to read |
| `mergeAllKey` | `bool` | `false` | Merge all keys as a map |
| `format` | `parser` | inferred | Parser override |
| `priority` | `source.Priority` | `PriorityRemote` | Source priority |
| `watch` | `bool` | `false` | Enable watch updates |
| `kubeconfig` | `string` | empty | Local kubeconfig path |

## Examples

- [`example/resolver`](./example/resolver): resolver with EndpointSlice/Endpoints watch.
- [`example/config-source`](./example/config-source): ConfigMap source.
- [`example/secret-source`](./example/secret-source): Secret source.

## Troubleshooting

- **No endpoints discovered**: verify service name, namespace, and RBAC permissions.
- **Cannot create kube client locally**: set `kubeconfig` to your local kubeconfig path.
- **Config source not updating**: ensure `watch: true` and resource `metadata.name` matches `name`.
