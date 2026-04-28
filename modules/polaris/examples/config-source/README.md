# Polaris Config Source

This example seeds a YAML file into Polaris Config Center and loads it through
the Yggdrasil v3 declarative config source `kind: polaris`.

本示例先把 YAML 文件写入 Polaris 配置中心，再通过 Yggdrasil v3 声明式配置源
`kind: polaris` 读取该远端配置。

## Run / 运行

Start Polaris:

启动 Polaris：

```bash
docker compose -f modules/polaris/examples/quickstart/compose.yaml up -d
```

Seed the remote config:

写入远端配置：

```bash
cd modules/polaris/examples/config-source/seed-config
go run .
```

Run the app:

运行示例应用：

```bash
cd modules/polaris/examples/config-source/app
go run .
```

Expected output:

预期输出：

```text
hello from Polaris config source, remote
```

## Configuration / 配置说明

- `seed-config/remote-config.yaml` is the content published to Polaris.
- `app/config.yaml` keeps only bootstrap Polaris SDK settings and declares
  `yggdrasil.config.sources` with `kind: polaris`.
- The remote layer has `priority: remote`, so it overrides local file values
  and stays below env/flag overrides.

- `seed-config/remote-config.yaml` 是写入 Polaris 的配置内容。
- `app/config.yaml` 只保留 Polaris SDK 的本地引导配置，并在
  `yggdrasil.config.sources` 下声明 `kind: polaris`。
- 远端配置层使用 `priority: remote`，因此会覆盖本地文件值，但仍低于环境变量和命令行。

## Troubleshooting / 常见问题

- If seeding fails, confirm `curl http://127.0.0.1:8090` returns
  `Polaris Server` and that port `8093` is mapped.
- If the app prints `hello from local fallback`, the remote file was not created
  or published. Run `seed-config` again and retry the app.

- 如果写入失败，确认 `curl http://127.0.0.1:8090` 返回 `Polaris Server`，
  且 `8093` 端口已映射。
- 如果应用输出 `hello from local fallback`，说明远端文件未创建或未发布；
  重新运行 `seed-config` 后再启动应用。
