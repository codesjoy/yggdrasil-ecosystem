# Polaris Metadata Discovery

This example registers two server instances with different metadata and uses the
Polaris resolver `metadata` filter to discover only one version.

本示例注册两个带不同 metadata 的服务实例，并通过 Polaris resolver 的
`metadata` 配置只发现指定版本。

## Run / 运行

Start Polaris:

启动 Polaris：

```bash
docker compose -f modules/polaris/examples/quickstart/compose.yaml up -d
```

Start the stable and canary servers in two terminals:

分别在两个终端启动 stable 和 canary 服务端：

```bash
cd modules/polaris/examples/metadata-discovery/server
go run . config-stable.yaml
```

```bash
cd modules/polaris/examples/metadata-discovery/server
go run . config-canary.yaml
```

Run the stable-filtered client:

运行只发现 stable 实例的客户端：

```bash
cd modules/polaris/examples/metadata-discovery/client
go run . config-stable.yaml
```

Expected output only contains `stable`:

预期输出只包含 `stable`：

```text
hello from stable [stable] to metadata-1
```

Run the canary-filtered client:

运行只发现 canary 实例的客户端：

```bash
cd modules/polaris/examples/metadata-discovery/client
go run . config-canary.yaml
```

Expected output only contains `canary`:

预期输出只包含 `canary`：

```text
hello from canary [canary] to metadata-1
```

## Notes / 说明

- Server metadata is declared under both `yggdrasil.admin.application.metadata`
  and `yggdrasil.transports.grpc.server.attr`.
- Client-side filtering is controlled by
  `yggdrasil.discovery.resolvers.polaris.config.metadata`.
- If the client reports no available instance, confirm that the metadata value
  in the client config exactly matches one registered instance.

- 服务端 metadata 同时写在 `yggdrasil.admin.application.metadata` 和
  `yggdrasil.transports.grpc.server.attr` 下。
- 客户端筛选条件由
  `yggdrasil.discovery.resolvers.polaris.config.metadata` 控制。
- 如果客户端提示没有可用实例，确认 client 配置中的 metadata 值和已注册实例完全一致。
