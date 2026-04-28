# All In One Example / 一体化示例

This example seeds one remote config document, prints the merged app message,
registers one demo instance, and watches resolver updates for the same service.

这个示例会先写入一份远端配置，打印合并后的应用消息，再注册一个 demo 实例并监听同名服务的解析更新。

## Run / 运行

```bash
cd modules/etcd/examples/allinone
go run .
```

## Expected Behavior / 预期现象

- The example seeds `/examples/etcd/allinone/config.yaml` into etcd.
- It prints the resolved message from the merged app config.
- It registers one demo service instance.
- It logs resolver updates until the process receives `Ctrl+C`.

## Back To Index / 返回索引

See the shared examples guide in [../README.md](../README.md).
