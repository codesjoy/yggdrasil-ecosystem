# Configure source Blob mode example

This example demonstrates how to use etcd's blob mode as the configuration source, read the complete YAML configuration file from a single etcd key, and listen for configuration changes to implement hot updates.

## What you get

- Read the complete YAML configuration file from etcd (stored in a single key)
- Configuration hot update: When the configuration in etcd changes, the application automatically receives notification and updates the configuration in memory
- Real-time demonstration: Automatically update the configuration every 10 seconds to demonstrate the monitoring effect of configuration changes

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
cd integrations/etcd/example/config-source/blob
go run main.go
```

## Expected output

```
2024/01/26 10:00:00 [etcd] initial config written to /example/config/blob
2024/01/26 10:00:00 [app] config source initialized, watching for changes...
2024/01/26 10:00:00 [app] press Ctrl+C to update config dynamically
2024/01/26 10:00:00 [app] press Ctrl+C again to exit
2024/01/26 10:00:10 [etcd] config updated (count=1)
2024/01/26 10:00:10 [config] server config updated: host=0.0.0.0, port=8081
2024/01/26 10:00:10 [config] database config updated: host=localhost, port=5432, name=mydb
2024/01/26 10:00:20 [etcd] config updated (count=2)
2024/01/26 10:00:20 [config] server config updated: host=0.0.0.0, port=8082
```

## Manually update configuration

You can also manually use `etcdctl` to update the configuration and observe whether the application receives change notifications:

```bash
# View current configuration
etcdctl --endpoints=127.0.0.1:2379 get /example/config/blob

# Update configuration
etcdctl --endpoints=127.0.0.1:2379 put /example/config/blob 'server:
  port: 9090
  host: "0.0.0.0"
database:
  host: "db.example.com"
  port: 3306
  name: "mydb"'
```

## Configuration instructions

### Blob mode features

- **Single key storage**: The entire configuration file is stored in one etcd key
- **Suitable for large configurations**: Suitable for scenarios where the configuration is large but updates are infrequent
- **Atomic Update**: Configuration updates are atomic and there will be no problem with partial updates.
- **Format Parsing**: Supports yaml/json/toml format, automatically parsed according to extension or configuration

### Configure source configuration

| Parameters | Description | Default value |
|------|------|--------|
| `mode` | Mode: blob or kv | `blob` |
| `key` | etcd key for configuration storage | Required |
| `watch` | Whether to monitor configuration changes | `true` |
| `format` | Configuration format: yaml/json/toml | `yaml` |

### Code structure description

```go
// 1. Create etcd client
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"127.0.0.1:2379"},
    DialTimeout: 5 * time.Second,
})

// 2. Write initial configuration to etcd
_, err = cli.Put(ctx, "/example/config/blob", initialConfig)

// 3. Create configuration source
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Client: etcd.ClientConfig{
        Endpoints:   []string{"127.0.0.1:2379"},
        DialTimeout: 5 * time.Second,
    },
    Mode:   etcd.ConfigSourceModeBlob,
    Key:    "/example/config/blob",
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
            Port int    `mapstructure:"port"`
            Host string `mapstructure:"host"`
        }
        if err := ev.Value().Scan(&serverConfig); err != nil {
            log.Printf("[config] failed to scan server config: %v", err)
            return
        }
        log.Printf("[config] server config updated: host=%s, port=%d", serverConfig.Host, serverConfig.Port)
    }
})
```

## Differences from KV mode

| Features | Blob Mode | KV Mode |
|------|-----------|---------|
| Storage | A single key stores the entire configuration | Multiple keys store configuration fragments |
| Suitable for scenarios | The configuration is large and updates are infrequent | The configuration is small and fine-grained updates are required |
| Update granularity | Overall update | A configuration item can be updated individually |
| Parsing method | Parse the entire configuration at one time | Need to merge the values ​​​​of multiple keys |
| Number of etcd keys | 1 | N (depending on the configuration level) |

## FAQ

**Q: Didn’t receive notification after configuration update? **

A: Check the following points:
1. Confirm that `watch: true` is configured
2. Confirm that the configuration key is correct (consistent with the key in the code)
3. Check whether the listener is correctly registered in `AddWatcher`

**Q: What should I do if the configuration file is too large? **

A: Blob mode is suitable for larger configuration scenarios, but it is recommended:
1. A single configuration does not exceed 1.5MB (etcd limit)
2. If the configuration is really large, consider splitting it into multiple configuration sources or using KV mode

**Q: How to support JSON/TOML format? **

A: Modify `Format` parameters:
```go
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    // ...
    Format: json.Unmarshal,  // or toml.Unmarshal
})
```

## Advanced usage

### Multiple configuration sources

You can use multiple configuration sources at the same time, with priority from high to low:

```go
// 1. Load local configuration file (low priority)
fileSrc := file.NewSource("./config.yaml", false)
config.LoadSource(fileSrc)

// 2. Load etcd configuration source (high priority, overwrite local configuration)
etcdSrc, _ := etcd.NewConfigSource(etcd.ConfigSourceConfig{...})
config.LoadSource(etcdSrc)
```

### Configure encryption

For sensitive configurations (such as database passwords), it is recommended to:
1. Use etcd’s encryption function
2. Or decrypt the configuration at the application layer
3. Or use a key management service (such as HashiCorp Vault)

## Related documents

- [etcd main documentation](../../../README.md)
- [KV mode example](../kv/)
- [Registry example](../../registry/)
- [Service Discovery Example](../../resolver/)
