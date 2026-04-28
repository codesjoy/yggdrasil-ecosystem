# Registry Example / 注册中心示例

This example builds the configured etcd registry from
`yggdrasil.discovery.registry` and registers one local demo instance.

这个示例会从 `yggdrasil.discovery.registry` 构建 etcd 注册中心，并注册一个本地 demo 实例。

## Run / 运行

```bash
cd modules/etcd/examples/registry
go run .
```

## Expected Behavior / 预期现象

- The example creates one local demo instance with `grpc` and `http` endpoints.
- It registers that instance into etcd with lease keepalive enabled.
- The process stays alive until it receives `Ctrl+C`, then deregisters on exit.

## Back To Index / 返回索引

See the shared examples guide in [../README.md](../README.md).
