# Configure source KV mode example

This example demonstrates how to use etcd's KV mode as a configuration source, read configuration fragments from multiple etcd keys, and map them into structured configurations based on the path hierarchy, supporting configuration hot updates.

## What you get

- Read multiple configuration keys from etcd prefix and automatically map them into a hierarchical structure
- Fine-grained configuration update: a configuration item can be updated individually without updating the entire configuration file
- Configuration hot update: When the configuration in etcd changes, the application automatically receives notifications
- Real-time demonstration: Automatically update part of the configuration every 10 seconds to demonstrate the advantages of KV mode

## Preconditions

1. Accessible etcd cluster (default `127.0.0.1:2379`)
2. Go 1.19+ installed

### Quickly start etcd

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

Verify that etcd is running properly:
```bash
etcdctl --endpoints=127.0.0.1:2379 endpoint health
```

## Startup method

### 1. Modify configuration (optional)

If the etcd address is not the default `127.0.0.1:2379`, please modify the `Endpoints` configuration in [main.go](main.go):

```go
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"your-etcd-endpoint:2379"},
    DialTimeout: 5 * time.Second,
})
```

### 2. Run the example

```bash
cd integrations/etcd/example/config-source/kv
go run main.go
```

## Expected output

```
2024/01/26 10:00:00 [etcd] initial config written to /example/config/kv/*
2024/01/26 10:00:00 [app] config source initialized, watching for changes...
2024/01/26 10:00:00 [app] press Ctrl+C to update config dynamically
2024/01/26 10:00:00 [app] press Ctrl+C again to exit
2024/01/26 10:00:10 [etcd] partial config updated (count=1)
2024/01/26 10:00:10 [config] server config updated: host=0.0.0.0, port=8081, timeout=30s
2024/01/26 10:00:10 [config] database config updated: host=localhost, port=5432, name=mydb, pool=11
2024/01/26 10:00:10 [config] cache config updated: ttl=360 seconds
2024/01/26 10:00:20 [etcd] partial config updated (count=2)
2024/01/26 10:00:20 [config] server config updated: host=0.0.0.0, port=8082, timeout=30s
2024/01/26 10:00:20 [config] database config updated: host=localhost, port=5432, name=mydb, pool=12
```

## KV structure description

### Key-Value in etcd

```
/example/config/kv/server/port        -> "8080"
/example/config/kv/server/host        -> "0.0.0.0"
/example/config/kv/server/readTimeout -> "30s"
/example/config/kv/database/host      -> "localhost"
/example/config/kv/database/port      -> "5432"
/example/config/kv/database/name      -> "mydb"
/example/config/kv/database/poolSize  -> "10"
/example/config/kv/logging/level     -> "info"
/example/config/kv/logging/format    -> "json"
/example/config/kv/cache/redis/host  -> "localhost"
/example/config/kv/cache/redis/port  -> "6379"
/example/config/kv/cache/ttl         -> "300"
```

###Mapped structure

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  readTimeout: "30s"
database:
  host: "localhost"
  port: 5432
  name: "mydb"
  poolSize: 10
logging:
  level: "info"
  format: "json"
cache:
  redis:
    host: "localhost"
    port: 6379
  ttl: 300
```

## Manually update configuration

You can manually use `etcdctl` to update a single configuration item and observe whether the application receives change notifications:

```bash
# View all configurations
etcdctl --endpoints=127.0.0.1:2379 get /example/config/kv --prefix

# Update a single configuration item (only update server/port)
etcdctl --endpoints=127.0.0.1:2379 put /example/config/kv/server/port "9090"

# Add new configuration item
etcdctl --endpoints=127.0.0.1:2379 put /example/config/kv/server/writeTimeout "60s"

# Delete configuration item
etcdctl --endpoints=127.0.0.1:2379 del /example/config/kv/cache/ttl
```

## Configuration instructions

### KV mode features

- **Multiple key storage**: configuration is scattered in multiple etcd keys
- **Path Mapping**: The path of etcd key is automatically mapped to the configured hierarchical structure
- **Fine-grained update**: You can update a configuration item individually without updating the entire configuration.
- **Suitable for small configurations**: Suitable for scenarios where the configuration is small but requires frequent updates

### Key naming rules

- **Path Separator**: Use `/` as path separator
- **Prefix matching**: All keys starting with `prefix` will be read
- **Auto mapping**: `/prefix/a/b/c` is mapped to `a.b.c`

### Configure source configuration

| Parameters | Description | Default value |
|------|------|--------|
| `mode` | Mode: blob or kv | `kv` |
| `prefix` | Configuration prefix | Required |
| `watch` | Whether to monitor configuration changes | `true` |
| `format` | Configuration format: yaml/json/toml | Automatic inference |

### Code structure description

```go
// 1. Create etcd client
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"127.0.0.1:2379"},
    DialTimeout: 5 * time.Second,
})

// 2. Write initial configuration to etcd (multiple keys)
ops := []clientv3.Op{
    clientv3.OpPut("/example/config/kv/server/port", "8080"),
    clientv3.OpPut("/example/config/kv/server/host", "0.0.0.0"),
    clientv3.OpPut("/example/config/kv/database/host", "localhost"),
    clientv3.OpPut("/example/config/kv/database/port", "5432"),
    clientv3.OpPut("/example/config/kv/database/name", "mydb"),
}
_, err = cli.Txn(ctx).Then(ops...).Commit()

// 3. Create configuration source (KV mode)
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Client: etcd.ClientConfig{
        Endpoints:   []string{"127.0.0.1:2379"},
        DialTimeout: 5 * time.Second,
    },
    Mode:   etcd.ConfigSourceModeKV,
    Prefix: "/example/config/kv",
    Watch:  boolPtr(true),
})

// 4. Load the configuration source into the framework
if err := config.LoadSource(cfgSrc); err != nil {
    log.Fatalf("config.LoadSource: %v", err)
}

// 5. Add configuration change listener
_ = config.AddWatcher("server", func(ev config.WatchEvent) {
    if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
        var serverConfig struct {
            Port         int    `mapstructure:"port"`
            Host         string `mapstructure:"host"`
            ReadTimeout  string `mapstructure:"readTimeout"`
        }
        if err := ev.Value().Scan(&serverConfig); err != nil {
            log.Printf("[config] failed to scan server config: %v", err)
            return
        }
        log.Printf("[config] server config updated: host=%s, port=%d",
            serverConfig.Host, serverConfig.Port)
    }
})
```

## Differences from Blob mode

| Features | KV Mode | Blob Mode |
|------|---------|-----------|
| Storage | Multiple keys store configuration fragments | A single key stores the entire configuration |
| Suitable for scenarios | The configuration is small and requires fine-grained updates | The configuration is large and updates are infrequent |
| Update granularity | A configuration item can be updated individually | Overall update |
| Parsing method | Need to merge the values ​​​​of multiple keys | Parse the entire configuration at one time |
| Number of etcd keys | N (depending on the configuration level) | 1 |
| Network overhead | Low overhead when updating a single configuration | Any update requires transferring the entire configuration |

## Advanced usage

### Dynamic configuration items

KV mode is particularly suitable for configuration items that need to be dynamically adjusted:

```go
// Adjust thread pool size at runtime
etcdctl put /example/config/kv/server/poolSize "20"

// Adjust cache expiration time at runtime
etcdctl put /example/config/kv/cache/ttl "600"

// Adjust log levels at runtime
etcdctl put /example/config/kv/logging/level "debug"
```

### Configuration inheritance and override

Configuration inheritance can be achieved through multiple prefixes:

```go
// 1. Load basic configuration (low priority)
baseSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Prefix: "/config/base",
    Priority: source.PriorityLocal,
})
config.LoadSource(baseSrc)

// 2. Load environment configuration (medium priority)
envSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Prefix: "/config/prod",
    Priority: source.PriorityRemote,
})
config.LoadSource(envSrc)

// 3. Load instance configuration (high priority, override other configurations)
instanceSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Prefix: "/config/instance-123",
    Priority: source.PriorityRemote + 1,
})
config.LoadSource(instanceSrc)
```

### Configure encryption

For sensitive configurations it is recommended:
1. Use etcd’s encryption function
2. Or decrypt the configuration at the application layer
3. Or use a key management service (such as HashiCorp Vault)

## FAQ

**Q: Why are some configuration items not read? **

A: Check the following points:
1. Confirm that the key prefix is ​​consistent with the `prefix` configuration
2. Confirm that the key path format is correct (separated by `/`)
3. Check whether there are illegal characters (`{}` is used for special processing)

**Q: How to update nested configuration? **

A: Directly update the corresponding key:
```bash
# Update cache.redis.host
etcdctl put /example/config/kv/cache/redis/host "redis.example.com"
```

**Q: How to deal with configurations of complex types (such as arrays)? **

A: Store JSON/YAML strings in a single key:
```bash
# Store array
etcdctl put /example/config/kv/server/servers '["10.0.0.1", "10.0.0.2", "10.0.0.3"]'

# storage object
etcdctl put /example/config/kv/server/metadata '{"env":"prod","region":"us-west"}'
```

Then parse it in code:
```go
var servers []string
config.Get("server.servers").Scan(&servers)

var metadata map[string]string
config.Get("server.metadata").Scan(&metadata)
```

**Q: How is the performance of KV mode? **

A: The performance of KV mode depends on the number of configuration items:
- Few configuration items (<100): excellent performance
- Large number of configuration items (>1000): It is recommended to consider using Blob mode or sharding

## Best Practices

1. **Properly design the key path**: Use a meaningful path structure, such as `/config/{app}/{module}/{key}`
2. **Control the number of configuration items**: Avoid creating too many configuration keys (recommended <100)
3. **Use namespace**: Different environments use different prefixes, such as `/config/dev`, `/config/prod`
4. **Monitor configuration changes**: Record configuration change logs to facilitate auditing and troubleshooting
5. **Configuration version management**: Include version information in the configuration to facilitate rollback

## Related documents

- [etcd main documentation](../../../README.md)
- [Blob pattern example](../blob/)
- [Registry example](../../registry/)
- [Service Discovery Example](../../resolver/)
