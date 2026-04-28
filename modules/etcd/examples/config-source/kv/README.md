# KV Config Source / KV 配置源示例

This example seeds multiple etcd keys under one prefix and loads them through
the declarative `kind: etcd` config source builder in prefix-based kv mode.

这个示例会先向同一前缀下写入多条 etcd key，再通过声明式 `kind: etcd` 配置源以前缀 kv 模式读取它们。

## Run / 运行

```bash
cd modules/etcd/examples/config-source/kv
go run .
```

## Expected Behavior / 预期现象

- The example seeds keys under `/examples/etcd/kv`.
- The local fallback config is merged with the remote kv tree.
- It prints the resolved greeting and name from the merged config.

## Back To Index / 返回索引

See the shared examples guide in [../../README.md](../../README.md).
