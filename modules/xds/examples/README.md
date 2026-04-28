# xDS Examples

This directory contains self-contained Yggdrasil v3 examples for the xDS
resolver and balancer module.

本目录提供一组自包含的 Yggdrasil v3 xDS resolver / balancer 示例。

## Layout / 目录结构

```text
examples/
├── cmd/control-plane/    # shared local xDS control-plane entry
├── internal/controlplane # shared control-plane implementation
├── internal/exampleapp   # shared client/server demo runtime
├── quickstart/           # single service, single endpoint
├── load-balancing/       # one service, three equal endpoints
├── canary/               # weighted clusters (95/5)
├── traffic-splitting/    # header-based route split
├── multi-service/        # two logical services in one xDS profile
├── helloworld/           # generated demo RPC bindings
└── proto/                # source proto for demo bindings
```

Each scenario keeps its own `README.md`, `client/`, `server/`, and `xds/`
directory. The only shared executable is `cmd/control-plane`.

每个场景目录都自带 `README.md`、`client/`、`server/` 和 `xds/`；唯一共享
可执行入口是 `cmd/control-plane`。

## Scenarios / 场景说明

| Scenario | What it demonstrates | Notes |
| --- | --- | --- |
| [quickstart](./quickstart/) | one listener, one cluster, one endpoint | shortest path to verify xDS discovery |
| [load-balancing](./load-balancing/) | one cluster with three endpoints | client repeats calls to observe instance rotation |
| [canary](./canary/) | weighted clusters with 95/5 split | stable and canary run as separate servers |
| [traffic-splitting](./traffic-splitting/) | header match routing with `x-release: canary` | includes default and canary client configs |
| [multi-service](./multi-service/) | two listeners and two logical services | one client calls both `library` and `greeter` |

## Control Plane / 控制面

Start the shared control plane from the examples module root:

从 examples 模块根目录启动共享控制面：

```bash
cd modules/xds/examples
go run ./cmd/control-plane \
  --bootstrap ./cmd/control-plane/bootstrap.yaml \
  --snapshot ./quickstart/xds/snapshot.yaml
```

Switch scenarios by replacing the `--snapshot` path with the scenario-local
`xds/snapshot.yaml`.

切换场景时，只需要把 `--snapshot` 换成目标场景自己的 `xds/snapshot.yaml`。

## Common Config Shape / 公共配置形态

Each client example uses the same xDS wiring:

每个 client 示例都复用同样的 xDS 接线方式：

- `yggdrasil.discovery.resolvers.xds`
- `yggdrasil.balancers.defaults.xds`
- `yggdrasil.clients.services.<target>.resolver/balancer`
- `yggdrasil.xds.default.config`

Each example also defines a small `app.example` section used by the shared demo
runtime:

每个示例额外定义一个很小的 `app.example` 配置段，供共享 demo 运行时读取：

- server: `service`, `greeting`, `instance`
- client: `services`, `requests`, `name`, `headers`

## Troubleshooting / 常见问题

- If the client reports no available instance, confirm the control plane is
  running on `127.0.0.1:18000` and that the selected scenario snapshot matches
  the server processes you started.
- If a scenario needs more than one server instance, make sure each instance is
  started with the correct config file from that scenario directory.
- If the control plane does not reload after editing a snapshot file, restart it
  and confirm the `--snapshot` path points to the file you changed.

- 如果客户端提示没有可用实例，先确认控制面运行在 `127.0.0.1:18000`，并且
  当前 `--snapshot` 与你启动的 server 进程一致。
- 如果某个场景需要多个 server 实例，确认每个实例都使用了该场景目录下正确的
  配置文件。
- 如果修改 snapshot 后控制面没有热更新，重新启动控制面，并确认 `--snapshot`
  指向的正是你修改的那个文件。
