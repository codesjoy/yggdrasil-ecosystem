# Kubernetes Resolver Example

This example prepares a Yggdrasil v3 app, builds the configured Kubernetes
resolver from the runtime snapshot, and prints endpoint updates for
`downstream-service`.

本示例会准备一个 Yggdrasil v3 app，从 runtime snapshot 中构建已配置的
Kubernetes resolver，并把 `downstream-service` 的 endpoint 变化打印到
stdout。

## Files / 文件

- `config.yaml`: resolver config used by the example.
- `manifests/deployment.yaml`: demo Deployment.
- `manifests/service.yaml`: demo Service.
- `main.go`: local resolver watcher that prints endpoint updates.

- `config.yaml`：示例使用的 resolver 配置。
- `manifests/deployment.yaml`：演示用 Deployment。
- `manifests/service.yaml`：演示用 Service。
- `main.go`：本地 resolver watcher，会打印 endpoint 更新。

## Prerequisites / 前置条件

- An accessible Kubernetes cluster.
- `kubectl` configured against that cluster.
- Go 1.25+.
- If you run outside the cluster, set `kubeconfig` in `config.yaml`.

- 一个可访问的 Kubernetes 集群。
- 已经能访问该集群的 `kubectl`。
- Go 1.25+。
- 如果你在集群外运行，请在 `config.yaml` 中设置 `kubeconfig`。

Quick cluster checks:

快速检查集群：

```bash
kubectl cluster-info
kubectl get nodes
```

## Run / 运行

Apply the demo workload:

先部署演示工作负载：

```bash
cd modules/k8s/examples/resolver
kubectl apply -f manifests/
```

Verify that the Service has endpoints:

确认 Service 已经生成 endpoints：

```bash
kubectl get svc downstream-service -n default
kubectl get endpoints downstream-service -n default -o wide
kubectl get endpointslices -n default -l kubernetes.io/service-name=downstream-service
```

Run the example:

启动示例：

```bash
go run .
```

## Expected Output / 预期输出

You should first see the watcher start log, then one resolver update that lists
the current endpoints:

启动后先看到 watcher 启动日志，随后出现一条 resolver update，列出当前
endpoints：

```text
resolver update for downstream-service: 3 endpoint(s)
- http://10.244.0.11:80 node=minikube zone=-
- http://10.244.0.12:80 node=minikube zone=-
- http://10.244.0.13:80 node=minikube zone=-
```

Exact IPs and node names will differ by cluster.

实际 IP 和 node 名称会随集群环境不同而变化。

## Update Verification / 变更验证

Scale the Deployment up:

扩容 Deployment：

```bash
kubectl scale deployment downstream-service --replicas=5 -n default
```

The running example should print a new update with 5 endpoints.

运行中的示例应打印一条新的更新，显示 5 个 endpoints。

Scale it back down:

再缩容：

```bash
kubectl scale deployment downstream-service --replicas=2 -n default
```

The example should print another update with 2 endpoints.

示例应再次打印更新，显示 2 个 endpoints。

## Troubleshooting / 排障

- If you only see `0 endpoint(s)`, verify the Service name, namespace, and port
  name in `config.yaml` match the manifests under `manifests/`.
- If the example cannot connect to Kubernetes from your laptop, set
  `yggdrasil.discovery.resolvers.kubernetes.config.kubeconfig` in `config.yaml`
  to your local kubeconfig path.
- If the cluster does not expose `EndpointSlice`, switch `mode` to
  `endpoints` and rerun.

- 如果你只看到 `0 endpoint(s)`，先确认 `config.yaml` 中的 Service 名称、
  namespace 和 port name 与 `manifests/` 下的清单一致。
- 如果示例在本机无法连接 Kubernetes，请在 `config.yaml` 中设置
  `yggdrasil.discovery.resolvers.kubernetes.config.kubeconfig` 为本地
  kubeconfig 路径。
- 如果集群没有可用的 `EndpointSlice`，把 `mode` 改成 `endpoints` 后重试。
