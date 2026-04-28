# Polaris Multi-Instance

This example starts two Yggdrasil v3 server instances under the same Polaris
service name. The client calls the service several times through the Polaris
resolver and balancer.

本示例用同一个 Polaris 服务名启动两个 Yggdrasil v3 服务实例，客户端通过
Polaris resolver 和 balancer 连续发起多次调用。

## Run / 运行

Start Polaris:

启动 Polaris：

```bash
docker compose -f modules/polaris/examples/quickstart/compose.yaml up -d
```

Start instance A:

启动实例 A：

```bash
cd modules/polaris/examples/multi-instance/server
go run . config-a.yaml
```

Start instance B in another terminal:

另开终端启动实例 B：

```bash
cd modules/polaris/examples/multi-instance/server
go run . config-b.yaml
```

Run the client from a third terminal:

第三个终端运行客户端：

```bash
cd modules/polaris/examples/multi-instance/client
go run .
```

Expected output contains responses from both instances:

预期输出会同时包含两个实例的响应：

```text
hello from instance A from a to request-1
hello from instance B from b to request-2
```

## Notes / 说明

- Both server configs use the same app name and register different gRPC
  addresses: `127.0.0.1:56131` and `127.0.0.1:56132`.
- The resolver refresh interval is three seconds. If only one instance appears,
  wait a few seconds and rerun the client.
- In the Polaris console, check the `default` namespace and the service named
  `github.com.codesjoy.yggdrasil-ecosystem.modules.polaris.examples.multi-instance.server`.

- 两个 server 配置使用同一个 app name，只是注册了不同的 gRPC 地址：
  `127.0.0.1:56131` 和 `127.0.0.1:56132`。
- resolver 每三秒刷新一次；如果只看到一个实例，等待几秒后重新运行客户端。
- 在 Polaris 控制台的 `default` 命名空间中检查
  `github.com.codesjoy.yggdrasil-ecosystem.modules.polaris.examples.multi-instance.server`
  服务。
