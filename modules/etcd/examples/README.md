# Etcd v3 Examples / Etcd v3 示例

These examples show how the Yggdrasil v3 etcd module handles remote config,
service registration, and service discovery.

这些示例展示了 Yggdrasil v3 etcd 模块如何处理远端配置、服务注册和服务发现。

## Prerequisites / 前置条件

- Go 1.25+
- A local etcd server reachable at `127.0.0.1:2379`

Quick start for etcd:

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

## Example Matrix / 示例矩阵

| Scenario | Reads Remote Config | Registers Instances | Resolves Instances | Best First Step |
| --- | --- | --- | --- | --- |
| [allinone](./allinone) | yes | yes | yes | Start here for the full flow |
| [config-source/blob](./config-source/blob) | yes, single key | no | no | Start here for remote config basics |
| [config-source/kv](./config-source/kv) | yes, prefix tree | no | no | Use after blob mode |
| [registry](./registry) | no | yes | no | Start here for lease-backed registration |
| [resolver](./resolver) | no | yes, demo self-registration | yes | Start here for watch-based discovery |

## Recommended Path / 推荐学习路径

1. [allinone](./allinone)
2. [config-source/blob](./config-source/blob)
3. [config-source/kv](./config-source/kv)
4. [registry](./registry)
5. [resolver](./resolver)

## Running Convention / 统一运行方式

Enter the scenario directory and run:

进入对应目录后执行：

```bash
go run .
```

Examples:

```bash
cd modules/etcd/examples/allinone
go run .

cd modules/etcd/examples/registry
go run .
```

Notes:

说明：

- `config-source/blob`, `config-source/kv`, and `allinone` seed etcd keys
  before preparing the Yggdrasil app.
- `registry` registers one demo instance and keeps it alive until the process
  receives a stop signal.
- `resolver` registers one demo instance, starts a resolver watch, and prints
  endpoint updates.

## Troubleshooting / 排障建议

- etcd is not running:
  Start or verify the local server with `etcdctl --endpoints=127.0.0.1:2379 endpoint health`.
- The endpoint is wrong:
  Update the example `config.yaml` if your etcd server is not on `127.0.0.1:2379`.
- Resolver shows no updates:
  Make sure registry and resolver use the same `prefix`, `namespace`, and
  compatible `protocols`.
- Config watch appears idle:
  Confirm the example seeded the expected `key` or `prefix`, and that `watch`
  remains enabled in the config source definition.
