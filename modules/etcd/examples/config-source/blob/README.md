# Blob Config Source / Blob 配置源示例

This example seeds one YAML document into etcd and loads it through the
declarative `kind: etcd` config source builder in single-key blob mode.

这个示例会先向 etcd 写入一份 YAML 文档，再通过声明式 `kind: etcd` 配置源以单 key blob 模式读取它。

## Run / 运行

```bash
cd modules/etcd/examples/config-source/blob
go run .
```

## Expected Behavior / 预期现象

- The example seeds `/examples/etcd/blob/config.yaml`.
- The local fallback config is merged with the remote blob document.
- It prints the resolved greeting and name from the merged config.

## Back To Index / 返回索引

See the shared examples guide in [../../README.md](../../README.md).
