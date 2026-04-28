# xDS Canary

This scenario uses real route-level `weighted_clusters`: `stable-cluster` gets
95% of traffic and `canary-cluster` gets 5%.

本场景使用真正的 route 级 `weighted_clusters`：`stable-cluster` 默认占 95%，
`canary-cluster` 占 5%。

## Run / 运行

Start the control plane:

启动控制面：

```bash
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./canary/xds/snapshot.yaml
```

Start stable and canary servers:

启动 stable 和 canary 服务端：

```bash
cd modules/xds/examples/canary/server
go run .
go run . config-canary.yaml
```

Run the client:

启动客户端：

```bash
cd modules/xds/examples/canary/client
go run .
```

Expected output:

预期输出：

- The client prints many responses and a final summary map.
- Most calls should hit `stable`; a smaller portion should hit `canary`.

- 客户端会打印多条响应以及最终汇总。
- 大多数请求应命中 `stable`，少量请求命中 `canary`。
