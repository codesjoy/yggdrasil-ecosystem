# Resolver Example / 服务发现示例

This example builds both an etcd registry and an etcd resolver, registers one
demo service instance, and prints resolver state updates for that service.

这个示例会同时构建 etcd 注册中心和 resolver，注册一个 demo 服务实例，并输出该服务的解析状态更新。

## Run / 运行

```bash
cd modules/etcd/examples/resolver
go run .
```

## Expected Behavior / 预期现象

- The example registers one demo instance for `example-registry-server`.
- It starts a resolver watch for that service name.
- It logs resolver updates and discovered endpoints until the process receives
  `Ctrl+C`.

## Back To Index / 返回索引

See the shared examples guide in [../README.md](../README.md).
