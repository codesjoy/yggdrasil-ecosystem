# Polaris Quickstart

This example runs one Yggdrasil v3 server, registers it with a local Polaris
standalone server, and calls it from a Yggdrasil v3 client through the Polaris
resolver and balancer.

本示例启动一个 Yggdrasil v3 服务端，将服务实例注册到本地 Polaris 单机版，
再由 Yggdrasil v3 客户端通过 Polaris resolver 和 balancer 发起调用。

## Prerequisites / 前置条件

- Docker with Compose support.
- Go matching this repository's `go.mod`.
- Ports `8080`, `8090`, `8091`, `8093`, `15010`, `8761`, `9000`, and `9090`
  available on localhost.

- 已安装支持 Compose 的 Docker。
- Go 版本与仓库 `go.mod` 匹配。
- 本机端口 `8080`、`8090`、`8091`、`8093`、`15010`、`8761`、`9000`、`9090`
  未被占用。

## Run / 运行

Start Polaris:

启动 Polaris：

```bash
docker compose -f modules/polaris/examples/quickstart/compose.yaml up -d
```

Check the Polaris console or API port:

检查 Polaris 控制台或 API 端口：

```bash
curl http://127.0.0.1:8090
```

Run the server from the server directory so `config.yaml` is loaded:

在服务端目录运行，确保加载同目录下的 `config.yaml`：

```bash
cd modules/polaris/examples/quickstart/server
go run .
```

Run the client from another terminal:

另开终端运行客户端：

```bash
cd modules/polaris/examples/quickstart/client
go run .
```

Expected output:

预期输出：

```text
hello from Polaris quickstart, quickstart
```

## Configuration / 配置说明

- `server/config.yaml` configures the gRPC listener, the Polaris SDK, and the
  default Polaris registry under `yggdrasil.discovery.registry`.
- `client/config.yaml` configures the Polaris SDK, resolver, Polaris balancer,
  and the client service entry for the server app name.
- Governance interceptors are wired in the client config, but rate limiting,
  circuit breaking, and routing are disabled by default for the quickstart path.

- `server/config.yaml` 配置 gRPC 监听地址、Polaris SDK，以及
  `yggdrasil.discovery.registry` 下的默认 Polaris 注册中心。
- `client/config.yaml` 配置 Polaris SDK、resolver、Polaris balancer，以及服务端
  app name 对应的客户端服务条目。
- 客户端配置已经接入治理拦截器，但 quickstart 默认关闭限流、熔断和路由规则，
  避免依赖额外 Polaris 治理配置。

## Troubleshooting / 常见问题

- If the client reports no available instance, wait a few seconds after the
  server starts and retry. The resolver polls Polaris every five seconds.
- If the server cannot register, confirm `127.0.0.1:8091` is reachable from the
  host and the Docker container is running.
- If `curl http://127.0.0.1:8090` fails, check whether another process already
  uses the mapped Polaris ports.

- 如果客户端提示没有可用实例，服务端启动后等待几秒再重试；resolver 每五秒轮询一次
  Polaris。
- 如果服务端注册失败，确认宿主机可以访问 `127.0.0.1:8091`，并且 Docker 容器正在运行。
- 如果 `curl http://127.0.0.1:8090` 失败，检查 Polaris 映射端口是否被其他进程占用。
