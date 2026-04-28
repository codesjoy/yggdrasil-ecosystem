# Kubernetes v3 Examples

This directory keeps self-contained Yggdrasil v3 examples for the Kubernetes
module.

本目录保存 Kubernetes 模块的自包含 Yggdrasil v3 示例。

## Layout / 目录结构

```text
examples/
├── resolver/
│   ├── config.yaml
│   ├── main.go
│   └── manifests/
├── config-source/
│   ├── config.yaml
│   ├── main.go
│   └── manifests/
└── secret-source/
    ├── config.yaml
    ├── main.go
    └── manifests/
```

Each example owns its own cluster resources under `manifests/`, so the runbook
for that scenario can use exact `kubectl apply -f ...` commands without
referencing external files.

每个示例都把自己用到的集群资源放在 `manifests/` 下，这样对应 runbook 可以
直接给出精确的 `kubectl apply -f ...` 命令，而不依赖外部文件。

## Choose an Example / 如何选择示例

| Example | What it proves | Notes |
| --- | --- | --- |
| [resolver](./resolver/) | Kubernetes resolver can watch Service endpoints and print updates | best choice when validating EndpointSlice/Endpoints discovery |
| [config-source](./config-source/) | ConfigMap-backed remote layer overrides local fallback config | one-shot composition example, rerun after patching ConfigMap |
| [secret-source](./secret-source/) | Secret-backed remote layer overrides local fallback config | one-shot composition example, rerun after patching Secret |

| 示例 | 证明什么 | 说明 |
| --- | --- | --- |
| [resolver](./resolver/) | Kubernetes resolver 能 watch Service endpoint 并打印更新 | 适合验证 EndpointSlice/Endpoints 发现链路 |
| [config-source](./config-source/) | ConfigMap 远端配置层能覆盖本地 fallback 配置 | 一次性 composition 示例，更新 ConfigMap 后重新运行即可验证 |
| [secret-source](./secret-source/) | Secret 远端配置层能覆盖本地 fallback 配置 | 一次性 composition 示例，更新 Secret 后重新运行即可验证 |

## Shared Prerequisites / 共享前置条件

- An accessible Kubernetes cluster.
- `kubectl` configured for that cluster.
- Go 1.25+.
- If you run from outside the cluster, set the example `kubeconfig` field.

- 一个可访问的 Kubernetes 集群。
- 已配置好该集群访问权限的 `kubectl`。
- Go 1.25+。
- 如果你在集群外运行，请设置示例配置里的 `kubeconfig` 字段。

Useful quick checks:

常用快速检查命令：

```bash
kubectl cluster-info
kubectl get nodes
```
