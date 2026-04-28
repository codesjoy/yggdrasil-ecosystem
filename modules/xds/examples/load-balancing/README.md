# xDS Load Balancing

This scenario keeps one logical service but publishes three equal endpoints in
xDS. The client sends repeated requests so you can observe instance rotation.

本场景保持一个逻辑服务，但在 xDS 中发布三个等权 endpoint；客户端会连续发起
多次调用，方便观察实例轮转。

## Run / 运行

Start the control plane:

启动控制面：

```bash
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./load-balancing/xds/snapshot.yaml
```

Start the three server instances in separate terminals:

分别在三个终端启动 server：

```bash
cd modules/xds/examples/load-balancing/server
go run .
go run . config-b.yaml
go run . config-c.yaml
```

Run the client:

启动客户端：

```bash
cd modules/xds/examples/load-balancing/client
go run .
```

Expected output shows a mixed summary such as:

预期输出会出现混合响应汇总，例如：

```text
responses=map[
  hello from xDS load balancing from instance-a, lb-1:4
  hello from xDS load balancing from instance-b, lb-2:4
  hello from xDS load balancing from instance-c, lb-3:4
]
```
