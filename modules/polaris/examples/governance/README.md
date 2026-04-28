# Polaris Governance Configuration

This is a configuration-only example for Polaris governance integration. It
shows how to enable routing, rate limiting, and circuit breaking from
Yggdrasil v3 client configuration.

这是一个配置型示例，展示如何在 Yggdrasil v3 客户端配置中开启 Polaris 路由、
限流和熔断能力。

## What It Demonstrates / 演示内容

- `routing.enable: true` lets the Polaris balancer call RouterAPI before
  selecting an instance.
- `rate_limit.enable: true` enables the `polaris_ratelimit` unary client
  interceptor and balancer-side quota checks.
- `circuit_breaker.enable: true` enables the `polaris_circuitbreaker` unary
  client interceptor and balancer-side circuit-breaker checks.
- `routing.arguments` and `rate_limit.arguments` provide custom labels such as
  `traffic=canary` to Polaris rules.

- `routing.enable: true` 会让 Polaris balancer 在选择实例前调用 RouterAPI。
- `rate_limit.enable: true` 会开启 `polaris_ratelimit` unary client
  interceptor，以及 balancer 侧的配额检查。
- `circuit_breaker.enable: true` 会开启 `polaris_circuitbreaker` unary client
  interceptor，以及 balancer 侧的熔断检查。
- `routing.arguments` 和 `rate_limit.arguments` 可向 Polaris 规则传入
  `traffic=canary` 这样的自定义标签。

## Usage / 使用方式

Use `client-config.yaml` as a reference when adapting `quickstart/client` or a
real service client. Before running with these switches enabled, create matching
route, rate-limit, and circuit-breaker rules in the Polaris console.

可将 `client-config.yaml` 作为改造 `quickstart/client` 或真实服务客户端的参考。
在启用这些开关前，需要先在 Polaris 控制台创建匹配的路由、限流和熔断规则。

## Notes / 说明

- This directory intentionally contains no runnable Go program. Governance
  behavior depends on Polaris-side rules, so it should not be part of normal
  `go test ./...`.
- If a rule is missing or too strict, requests can fail with resource exhausted,
  unavailable, or no available instance errors.

- 本目录故意不放可运行 Go 程序。治理行为依赖 Polaris 侧规则，不应进入普通
  `go test ./...` 的自动化路径。
- 如果规则缺失或过于严格，请求可能返回资源耗尽、不可用或无可用实例等错误。
