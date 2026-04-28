# Kubernetes Secret Source Example

This example loads one remote config layer through
`yggdrasil.config.sources` with `kind: kubernetes-secret` and prints the
resolved `app.config_source` values.

本示例通过 `yggdrasil.config.sources` 下的
`kind: kubernetes-secret` 加载远端配置层，并打印最终解析出的
`app.config_source` 配置值。

## Files / 文件

- `config.yaml`: local bootstrap config plus local fallback values.
- `manifests/secret.yaml`: initial remote Secret.
- `manifests/secret-updated.yaml`: updated Secret for verification.

- `config.yaml`：本地引导配置以及本地 fallback 值。
- `manifests/secret.yaml`：初始远端 Secret。
- `manifests/secret-updated.yaml`：用于验证更新效果的新版 Secret。

## Prerequisites / 前置条件

- An accessible Kubernetes cluster.
- `kubectl` configured against that cluster.
- Go 1.25+.
- If you run outside the cluster, set `kubeconfig` in `config.yaml`.

- 一个可访问的 Kubernetes 集群。
- 已经能访问该集群的 `kubectl`。
- Go 1.25+。
- 如果你在集群外运行，请在 `config.yaml` 中设置 `kubeconfig`。

## Run / 运行

Create the initial Secret:

先创建初始 Secret：

```bash
cd modules/k8s/examples/secret-source
kubectl apply -f manifests/secret.yaml
```

Verify the resource:

确认资源存在：

```bash
kubectl get secret example-secret -n default -o yaml
```

Run the example:

启动示例：

```bash
go run .
```

## Expected Output / 预期输出

The remote layer should override the local fallback and print:

远端配置层应覆盖本地 fallback，输出：

```text
hello from Secret, remote
```

## Update Verification / 更新验证

Apply the updated Secret:

应用更新后的 Secret：

```bash
kubectl apply -f manifests/secret-updated.yaml
```

Run the example again:

再次运行示例：

```bash
go run .
```

Expected output:

预期输出：

```text
hello from Secret v2, remote-updated
```

Like the ConfigMap example, this sample exits after one composition. It verifies
that the remote Secret layer is read correctly and overrides the local fallback.

和 ConfigMap 示例一样，这个样例在一次 composition 后就退出；它用于验证
远端 Secret 配置层能够被正确读取，并覆盖本地 fallback。

## Troubleshooting / 排障

- If you still see `hello from local fallback, local`, confirm the Secret key is
  named `config.yaml` and contains an `app.config_source` section.
- Use `stringData` for demo manifests so you do not have to hand-encode the YAML
  as Base64.
- Treat this demo Secret as sample data only; do not commit real credentials.

- 如果你仍然看到 `hello from local fallback, local`，先确认 Secret 的 key 名称
  是 `config.yaml`，并且内容里包含 `app.config_source` 配置段。
- 示例 manifest 使用 `stringData`，这样你无需手工把 YAML 再编码成 Base64。
- 这个示例 Secret 只用于演示，请不要把真实凭证提交进版本库。
