# xDS Quickstart

This example is the shortest runnable path for the xDS module: one control
plane, one server, one client, and one endpoint.

本示例是 xDS 模块最短可运行路径：一个控制面、一个服务端、一个客户端、一个
endpoint。

## Run / 运行

Start the control plane from `modules/xds/examples`:

从 `modules/xds/examples` 启动控制面：

```bash
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./quickstart/xds/snapshot.yaml
```

Run the server:

启动服务端：

```bash
cd modules/xds/examples/quickstart/server
go run .
```

Run the client:

启动客户端：

```bash
cd modules/xds/examples/quickstart/client
go run .
```

Expected output contains:

预期输出包含：

```text
hello from xDS quickstart, quickstart-1
```
