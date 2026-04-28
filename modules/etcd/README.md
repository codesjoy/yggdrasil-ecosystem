# Etcd Module for Yggdrasil v3

Yggdrasil v3 的 etcd 模块，用于把 etcd 接入配置源、注册中心和服务发现能力。

This module integrates etcd with the Yggdrasil v3 module/capability runtime. It
provides:

- `etcd.Module()` for the `type: etcd` registry and resolver providers.
- `etcd.WithModule()` as the convenience bootstrap option.
- `kind: etcd` declarative config sources under `yggdrasil.config.sources`.
- Programmatic helpers `etcd.NewConfigSource(...)` and
  `etcd.WithConfigSource(...)`.

## Before You Run Examples / 运行示例前

The examples assume a local etcd server on `127.0.0.1:2379`.

示例默认依赖本地 etcd，地址为 `127.0.0.1:2379`。

Start one quickly:

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

Optional health check:

```bash
etcdctl --endpoints=127.0.0.1:2379 endpoint health
```

## Installation / 安装

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3
```

Register the module explicitly:

显式注册模块：

```go
app, err := yggdrasil.New(
    "example",
    etcd.WithModule(),
)
```

Blank-import side-effect registration is not supported in v3.

v3 不支持通过空白导入进行副作用注册。

## Configuration Model / 配置模型

The module uses named etcd clients under `yggdrasil.etcd.clients`.
Discovery providers keep the provider type name `etcd`, and declarative config
sources use `kind: etcd`.

模块通过 `yggdrasil.etcd.clients` 管理命名客户端；服务注册与发现继续使用
`type: etcd`，声明式配置源使用 `kind: etcd`。

```yaml
yggdrasil:
  etcd:
    clients:
      default:
        endpoints: ["127.0.0.1:2379"]
        dial_timeout: 5s
        username: ""
        password: ""

  discovery:
    registry:
      type: etcd
      config:
        client: default
        prefix: /yggdrasil/registry
        ttl: 10s
        keep_alive: true
        retry_interval: 3s

    resolvers:
      etcd:
        type: etcd
        config:
          client: default
          prefix: /yggdrasil/registry
          namespace: default
          protocols: [grpc, http]
          debounce: 200ms
```

### Client Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `endpoints` | `[]string` | `["127.0.0.1:2379"]` | etcd endpoints |
| `dial_timeout` | `duration` | `5s` | etcd dial timeout |
| `username` | `string` | empty | optional username |
| `password` | `string` | empty | optional password |

### Registry Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `client` | `string` | `default` | named etcd client |
| `prefix` | `string` | `/yggdrasil/registry` | registry key prefix |
| `ttl` | `duration` | `10s` | lease TTL |
| `keep_alive` | `bool` | `true` | enable lease keepalive |
| `retry_interval` | `duration` | `3s` | retry delay after keepalive failure |

### Resolver Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `client` | `string` | `default` | named etcd client |
| `prefix` | `string` | `/yggdrasil/registry` | registry prefix to watch |
| `namespace` | `string` | `default` | namespace filter |
| `protocols` | `[]string` | `[grpc,http]` | allowed endpoint protocols |
| `debounce` | `duration` | `0` | debounce window for watch events |

## Config Sources / 配置源

`etcd.WithModule()` registers a declarative source builder. Keep bootstrap
client settings in the local file, then declare the remote layer under
`yggdrasil.config.sources`:

`etcd.WithModule()` 会注册声明式配置源 builder。通常把 etcd 客户端放在本地
配置里，再把远端配置层声明到 `yggdrasil.config.sources`：

```yaml
yggdrasil:
  etcd:
    clients:
      default:
        endpoints: ["127.0.0.1:2379"]

  config:
    sources:
      - kind: etcd
        name: etcd:app
        priority: remote
        config:
          client: default
          key: /demo/app/config.yaml
          watch: true
          format: yaml
```

Programmatic usage is also available:

也可以通过编程方式挂载：

```go
src, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Client: "default",
    Key:    "/demo/app/config.yaml",
})
if err != nil {
    panic(err)
}

app, err := yggdrasil.New(
    "example",
    etcd.WithModule(),
    etcd.WithConfigSource("etcd:app", etcd.ConfigSourceConfig{
        Client: "default",
        Key:    "/demo/app/config.yaml",
    }),
)
```

Config source behavior:

配置源行为说明：

- Exactly one of `key` or `prefix` must be set.
- `mode` is optional and inferred automatically:
  - `key` set -> `blob`
  - `prefix` set -> `kv`
- `watch` defaults to enabled.
- `name` defaults to the explicit `name`, otherwise falls back to the source
  key or prefix.

## Choose The Right Example / 如何选择示例

- `config-source/blob`: load one full document from a single etcd key.
- `config-source/kv`: load one structured config tree from an etcd prefix.
- `registry`: register one demo service instance with etcd lease keepalive.
- `resolver`: watch one service and print resolver state updates.
- `allinone`: see config source, registry, and resolver in one runnable flow.

更多公共说明、运行约定和排障建议见 [examples/README.md](./examples/README.md)。

## Examples / 示例

Runnable examples live under [`examples/`](./examples/):

- [examples/allinone](./examples/allinone)
- [examples/config-source/blob](./examples/config-source/blob)
- [examples/config-source/kv](./examples/config-source/kv)
- [examples/registry](./examples/registry)
- [examples/resolver](./examples/resolver)

## Testing / 测试

This module keeps unit tests and integration-tagged package tests next to the
implementation. Current integration scenarios use embedded etcd, so Docker is
not required for automated tests:

当前自动化集成测试使用 embedded etcd，因此不需要 Docker：

```bash
make test TEST_TAGS=integration MODULES="modules/etcd"
```
