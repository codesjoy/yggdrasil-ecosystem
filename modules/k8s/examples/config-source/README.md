# Kubernetes ConfigMap Source Example

This example loads one remote config layer through
`yggdrasil.config.sources` with `kind: kubernetes-configmap` and prints the
resolved `app.config_source` values.

本示例通过 `yggdrasil.config.sources` 下的
`kind: kubernetes-configmap` 加载远端配置层，并打印最终解析出的
`app.config_source` 配置值。

## Files / 文件

- `config.yaml`: local bootstrap config plus local fallback values.
- `manifests/configmap.yaml`: initial remote config.
- `manifests/configmap-updated.yaml`: updated remote config for verification.

- `config.yaml`：本地引导配置以及本地 fallback 值。
- `manifests/configmap.yaml`：初始远端配置。
- `manifests/configmap-updated.yaml`：用于验证更新效果的新版远端配置。

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

Create the initial ConfigMap:

先创建初始 ConfigMap：

```bash
cd modules/k8s/examples/config-source
kubectl apply -f manifests/configmap.yaml
```

Verify the resource:

确认资源存在：

```bash
kubectl get configmap example-config -n default -o yaml
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
hello from ConfigMap, remote
```

## Update Verification / 更新验证

Apply the updated ConfigMap:

应用更新后的 ConfigMap：

```bash
kubectl apply -f manifests/configmap-updated.yaml
```

Run the example again:

再次运行示例：

```bash
go run .
```

Expected output:

预期输出：

```text
hello from ConfigMap v2, remote-updated
```

Note: this example exits after one composition. It demonstrates declarative
remote loading and precedence, not a long-running live reload loop.

注意：本示例在完成一次 composition 后就退出。它主要演示声明式远端配置加载
和优先级，不演示长生命周期进程中的实时热更新循环。

## Troubleshooting / 排障

- If you still see `hello from local fallback, local`, confirm the ConfigMap
  key is named `config.yaml` and contains an `app.config_source` section.
- If `kubectl get configmap` succeeds but the example cannot read it, check the
  namespace and `kubeconfig` settings.
- Keep `watch: true` in `config.yaml` for parity with production wiring, but do
  not expect this short-lived example to stay alive and stream updates.

- 如果你仍然看到 `hello from local fallback, local`，先确认 ConfigMap 的 key
  名称是 `config.yaml`，并且内容里包含 `app.config_source` 配置段。
- 如果 `kubectl get configmap` 成功但示例无法读取，请检查 namespace 和
  `kubeconfig` 设置。
- `config.yaml` 里保留 `watch: true` 是为了保持和生产 wiring 一致，但不要把
  这个短生命周期示例当成持续运行的热更新进程。
