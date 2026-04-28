# xDS Multi-Service

This scenario keeps one xDS profile but publishes two listeners and two logical
service targets: `library` and `greeter`.

本场景在同一个 xDS profile 下发布两个 listener 和两个逻辑服务目标：
`library` 与 `greeter`。

## Run / 运行

Start the control plane:

启动控制面：

```bash
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./multi-service/xds/snapshot.yaml
```

Start the two service processes:

启动两个服务进程：

```bash
cd modules/xds/examples/multi-service/server
go run .
go run . config-greeter.yaml
```

Run the client:

启动客户端：

```bash
cd modules/xds/examples/multi-service/client
go run .
```

Expected output contains one response for each target:

预期输出会同时包含两个目标的响应：

```text
service=library message="hello from xDS multi-service from library, multi-service-1"
service=greeter message="hello from xDS multi-service from greeter, multi-service-1"
```
