# xDS Traffic Splitting

This scenario routes by request header instead of weighted clusters. The default
route goes to `stable-cluster`; requests with `x-release: canary` go to
`canary-cluster`.

本场景通过请求头路由，而不是 weighted clusters。默认请求走
`stable-cluster`；携带 `x-release: canary` 的请求走 `canary-cluster`。

## Run / 运行

Start the control plane:

启动控制面：

```bash
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./traffic-splitting/xds/snapshot.yaml
```

Start stable and canary servers:

启动 stable 和 canary 服务端：

```bash
cd modules/xds/examples/traffic-splitting/server
go run .
go run . config-canary.yaml
```

Run the default client:

运行默认客户端：

```bash
cd modules/xds/examples/traffic-splitting/client
go run .
```

Run the canary client:

运行 canary 客户端：

```bash
cd modules/xds/examples/traffic-splitting/client
go run . config-canary.yaml
```

Expected behavior:

预期行为：

- default client: only stable responses
- canary client: only canary responses

- 默认客户端：只返回 stable 响应
- canary 客户端：只返回 canary 响应
