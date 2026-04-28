# Kubernetes Module for Yggdrasil v3

This module integrates Kubernetes with the Yggdrasil v3 module/capability
runtime.

本模块把 Kubernetes 接入到 Yggdrasil v3 的 module/capability runtime 中。

It provides:

它提供：

- `k8s.Module()` for the `type: kubernetes` discovery resolver.
- `k8s.WithModule()` as the convenience bootstrap option.
- Declarative config sources under `yggdrasil.config.sources` with
  `kind: kubernetes-configmap` and `kind: kubernetes-secret`.
- Programmatic helpers `NewConfigMapSource`, `NewSecretSource`,
  `WithConfigMapSource`, and `WithSecretSource`.

- `k8s.Module()`：注册 `type: kubernetes` discovery resolver。
- `k8s.WithModule()`：方便在 bootstrap 时直接挂载模块。
- 声明式配置源：在 `yggdrasil.config.sources` 下使用
  `kind: kubernetes-configmap` 和 `kind: kubernetes-secret`。
- 编程式 helper：`NewConfigMapSource`、`NewSecretSource`、
  `WithConfigMapSource`、`WithSecretSource`。

## Installation / 安装

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3
```

Register the module explicitly:

显式注册模块：

```go
app, err := yggdrasil.New(
    "example",
    k8s.WithModule(),
)
```

Blank-import side-effect registration is not supported in v3.

v3 不支持通过 blank import 做副作用注册。

## Resolver / Resolver 配置

The resolver keeps the legacy backend type name `kubernetes`, but the config
shape moves to the Yggdrasil v3 discovery tree:

resolver 继续沿用旧的 backend type 名称 `kubernetes`，但配置位置迁移到了
Yggdrasil v3 的 discovery 树下：

```yaml
yggdrasil:
  discovery:
    resolvers:
      my-k8s:
        type: kubernetes
        config:
          namespace: default
          mode: endpointslice
          port_name: grpc
          protocol: grpc
          kubeconfig: ""
          endpoint_attributes:
            cluster: prod-a
          backoff:
            base_delay: 1s
            multiplier: 1.6
            jitter: 0.2
            max_delay: 30s

  clients:
    services:
      my-service:
        resolver: my-k8s
        balancer: default
```

| Field / 字段 | Type | Default / 默认值 | Description / 说明 |
| --- | --- | --- | --- |
| `namespace` | `string` | `KUBERNETES_NAMESPACE` or `default` | Namespace to watch / 要 watch 的 namespace |
| `mode` | `string` | `endpointslice` | `endpointslice` or `endpoints`; EndpointSlice first, Endpoints fallback / 优先 EndpointSlice，失败后回退 Endpoints |
| `port_name` | `string` | empty | Preferred port name / 优先匹配的端口名 |
| `port` | `int32` | `0` | Fallback port number / 备用端口号 |
| `protocol` | `string` | `grpc` | Logical protocol label on resolved endpoints / 解析结果里写入的协议标签 |
| `kubeconfig` | `string` | empty | Local kubeconfig path; empty means in-cluster config / 本地 kubeconfig 路径；为空时走 in-cluster config |
| `endpoint_attributes` | `map[string]string` | nil | Extra attributes copied onto every endpoint / 追加到每个 endpoint 上的额外属性 |
| `backoff.base_delay` | `duration` | `1s` | Initial reconnect delay / 初始重试延迟 |
| `backoff.multiplier` | `float64` | `1.6` | Backoff multiplier / 退避倍数 |
| `backoff.jitter` | `float64` | `0.2` | Backoff jitter / 抖动系数 |
| `backoff.max_delay` | `duration` | `30s` | Maximum reconnect delay / 最大重试延迟 |
| `resync_period` | `duration` | `0` | Reserved field / 保留字段 |
| `timeout` | `duration` | `0` | Reserved field / 保留字段 |

Important behavior:

关键行为：

- `mode: endpointslice` falls back to `endpoints` if EndpointSlice watch/list
  setup fails.
- `port_name` takes precedence over `port`.
- On the Endpoints path, if neither `port_name` nor `port` is set, the first
  endpoint port is used.
- `protocol` is a logical endpoint label; it does not negotiate or validate the
  actual application protocol on the Service port.

- 当 `EndpointSlice` 的 watch/list 建立失败时，`mode: endpointslice` 会自动
  回退到 `endpoints`。
- `port_name` 的优先级高于 `port`。
- 在 Endpoints 路径下，如果 `port_name` 和 `port` 都没设置，就使用第一个
  endpoint port。
- `protocol` 只是 resolver state 上的逻辑标签，不负责协商或校验 Service 端口
  上真实跑的应用协议。

Useful cluster-side verification commands:

集群侧常用验证命令：

```bash
kubectl get svc <service> -n <namespace>
kubectl get endpoints <service> -n <namespace> -o wide
kubectl get endpointslices -n <namespace> -l kubernetes.io/service-name=<service>
```

## Config Sources / 配置源

### Declarative / 声明式

Keep Kubernetes bootstrap credentials in the local file, then declare the
remote layer under `yggdrasil.config.sources`:

把 Kubernetes 的引导连接信息保留在本地文件里，再在
`yggdrasil.config.sources` 下声明远端配置层：

```yaml
yggdrasil:
  config:
    sources:
      - kind: kubernetes-configmap
        name: k8s:app-config
        priority: remote
        config:
          namespace: default
          name: my-config
          key: config.yaml
          watch: true
```

Use `kind: kubernetes-secret` with the same config shape for Secret-backed
configuration.

如果要从 Secret 读取配置，把 `kind` 换成 `kubernetes-secret` 即可，配置结构
保持一致。

### Programmatic / 编程式

```go
src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Watch:     true,
})
if err != nil {
    panic(err)
}

app, err := yggdrasil.New(
    "example",
    k8s.WithModule(),
    yggdrasil.WithConfigSource("k8s:app-config", config.PriorityRemote, src),
)
```

| Field / 字段 | Type | Default / 默认值 | Description / 说明 |
| --- | --- | --- | --- |
| `namespace` | `string` | empty | Resource namespace; set it explicitly / 资源所在 namespace，建议显式填写 |
| `name` | `string` | required | ConfigMap or Secret name / ConfigMap 或 Secret 名称 |
| `key` | `string` | inferred | Key to read / 要读取的 key |
| `merge_all_keys` | `bool` | `false` | Merge all keys into one in-memory map / 把所有 key 合并成一个内存 map |
| `format` | `parser` | inferred | Parser override (`yaml`, `json`, `toml`) / 显式指定解析器 |
| `watch` | `bool` | `false` | Enable watch updates / 是否开启 watch 更新 |
| `kubeconfig` | `string` | empty | Local kubeconfig path / 本地 kubeconfig 路径 |

Important behavior:

关键行为：

- `namespace` for config sources is intentionally not defaulted from
  `KUBERNETES_NAMESPACE`; set it explicitly.
- `watch: true` makes the source implement `config/source.Watchable`, but you
  only observe hot reload in a long-running app that actually stays alive.
- For `merge_all_keys: true`, the payload is injected as a map instead of
  parsing one specific remote file.

- config source 的 `namespace` 不会从 `KUBERNETES_NAMESPACE` 自动补齐，建议你
  显式填写。
- `watch: true` 会让 source 实现 `config/source.Watchable`，但只有长生命周期、
  真正保持运行的 app 才能观察到热更新。
- 当 `merge_all_keys: true` 时，source 会把远端内容作为 map 注入，而不是解析
  某一个单独文件。

## RBAC / 权限

If you run inside Kubernetes with a service account, the minimum read-only
permissions are:

如果你以 in-cluster service account 的方式运行，最小只读权限如下：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: yggdrasil-k8s
  namespace: default
rules:
  - apiGroups: [""]
    resources: ["endpoints", "configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
```

When you run locally with your own kubeconfig, the effective permissions are the
permissions of that kubeconfig identity.

如果你是在本机通过 kubeconfig 运行，最终生效的是 kubeconfig 身份自身拥有的
权限。

## Examples / 示例

Runnable examples live under [`examples/`](./examples/):

可运行示例位于 [`examples/`](./examples/)：

- `resolver`: prints resolver state updates for one Kubernetes Service.
- `config-source`: proves that a ConfigMap-backed remote layer overrides local
  fallback config.
- `secret-source`: proves that a Secret-backed remote layer overrides local
  fallback config.

- `resolver`：打印某个 Kubernetes Service 的 resolver state 更新。
- `config-source`：验证 ConfigMap 远端配置层会覆盖本地 fallback 配置。
- `secret-source`：验证 Secret 远端配置层会覆盖本地 fallback 配置。

Each example keeps its own `manifests/` directory so the README can provide
exact `kubectl apply -f ...` steps.

每个示例都自带 `manifests/` 目录，因此 README 可以直接给出精确的
`kubectl apply -f ...` 步骤。

## Migration from Removed v2 / 从已移除的 v2 迁移

The old import path `github.com/codesjoy/yggdrasil-ecosystem/integrations/k8s/v2`
has been removed from this repository. There is no compatibility stub.

旧导入路径
`github.com/codesjoy/yggdrasil-ecosystem/integrations/k8s/v2`
已经从本仓库中移除，不再保留兼容 stub。

| v2 | v3 |
| --- | --- |
| blank import side effect registration | `k8s.WithModule()` explicit registration |
| `yggdrasil.resolver.<name>` | `yggdrasil.discovery.resolvers.<name>` |
| `portName` | `port_name` |
| `endpointAttributes` | `endpoint_attributes` |
| `resyncPeriod` | `resync_period` |
| `baseDelay` | `base_delay` |
| `maxDelay` | `max_delay` |
| `mergeAllKey` | `merge_all_keys` |
| `Priority` on `ConfigSourceConfig` | priority lives on `yggdrasil.config.sources[].priority` or `yggdrasil.WithConfigSource(...)` |

Recommended migration order:

建议迁移顺序：

1. Replace the old import with `modules/k8s/v3`.
2. Replace blank-import registration with `k8s.WithModule()`.
3. Move resolver config into `yggdrasil.discovery.resolvers`.
4. Rename camelCase config keys to snake_case.
5. Replace old ConfigMap/Secret source wiring with the new declarative kinds or
   the new programmatic helpers.

1. 先把旧 import 替换成 `modules/k8s/v3`。
2. 用 `k8s.WithModule()` 取代 blank-import 注册。
3. 把 resolver 配置迁移到 `yggdrasil.discovery.resolvers`。
4. 把 camelCase 配置键改成 snake_case。
5. 把旧的 ConfigMap/Secret source 接线切到新的声明式 kind 或新的 helper。

## Troubleshooting / 常见问题

- No endpoints discovered:
  check Service name, namespace, port name, and whether EndpointSlice is
  available in the cluster.
- Resolver starts but always returns zero endpoints:
  compare `kubectl get endpoints` / `kubectl get endpointslices` with your
  resolver config and make sure the Service really has ready Pods.
- Remote config source still returns local fallback values:
  verify the resource name, namespace, key name, and payload structure under
  `app.config_source`.
- Permission denied errors:
  grant `get/list/watch` on `endpoints`, `endpointslices`, `configmaps`, and
  `secrets`.

- 没有发现任何 endpoint：
  检查 Service 名称、namespace、port name，以及集群是否支持 EndpointSlice。
- resolver 启动了但一直返回零 endpoints：
  用 `kubectl get endpoints` / `kubectl get endpointslices` 对照 resolver
  配置，确认 Service 背后确实有 ready Pod。
- 远端配置源仍然返回本地 fallback：
  检查资源名称、namespace、key 名称，以及 `app.config_source` 这一层的配置结构。
- 出现 permission denied：
  需要授予 `endpoints`、`endpointslices`、`configmaps`、`secrets` 的
  `get/list/watch` 权限。
