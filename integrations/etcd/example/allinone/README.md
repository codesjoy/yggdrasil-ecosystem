# Complete example (All-in-One)

This example demonstrates all functions of etcd contrib module: configuration source, registry and service discovery, fully displayed in one application.

## What you get

- **Configuration Source**: Read configuration from etcd and listen for changes
- **Registration Center**: Register service instances to etcd and automatically maintain heartbeats
- **Service Discovery**: Discover service instances from etcd and monitor changes in real time
- **Full Process**: Demonstrates how the three components work together

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

If the etcd address is not the default `127.0.0.1:2379`, please modify [config.yaml](config.yaml):

```yaml
etcd:
  client:
    endpoints:
      - "your-etcd-endpoint:2379"
```

### 2. Run the example

```bash
cd integrations/etcd/example/allinone
go run main.go
```

## Expected output

```
2024/01/26 10:00:00 [registry] instance registered
2024/01/26 10:00:00 [config] updated: message: hello from etcd at 2024-01-26T10:00:00Z
2024/01/26 10:00:00 [resolver] endpoints: 1
2024/01/26 10:00:00   - grpc://127.0.0.1:9000
2024/01/26 10:00:05 [config] updated: message: hello from etcd at 2024-01-26T10:00:05Z
2024/01/26 10:00:10 [config] updated: message: hello from etcd at 2024-01-26T10:00:10Z
...
```

## Architecture description

```
┌─────────────────────────────────────────────────────────┐
│                 Application                          │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │ Config      │  │ Registry    │  │ Resolver    │ │
│  │ Source      │  │             │  │             │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘ │
│         │                │                │          │
└─────────┼────────────────┼────────────────┼──────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────────────────────────────────────────┐
│                    etcd                          │
│                                                     │
│ /demo/allinone/config (configuration source) │
│ /yggdrasil/registry/... (registry/discovery) │
└─────────────────────────────────────────────────────┘
```

## Detailed explanation of functions

### 1. Configuration source

- **Mode**: Blob mode, single key storage configuration
- **Key**：`/demo/allinone/config`
- **Watch**: Enable, automatically update configuration every 5 seconds
- **Listener**: Print the latest content when the configuration changes

```go
cfgSrc, err := etcd.NewConfigSource(etcd.ConfigSourceConfig{
    Mode:  etcd.ConfigSourceModeBlob,
    Key:   "/demo/allinone/config",
    Watch:  boolPtr(true),
})
config.LoadSource(cfgSrc)

config.AddWatcher("", func(ev config.WatchEvent) {
    if ev.Type() == config.WatchEventUpd {
        log.Printf("[config] updated: %s", string(ev.Value().Bytes()))
    }
})
```

### 2. Registration Center

- **Prefix**: `/yggdrasil/registry`
- **Service name**:`demo-allinone`
- **Version**: `1.0.0`
- **Protocol**: `grpc`
- **TTL**: 10 seconds
- **KeepAlive**: Enable, automatically maintain heartbeat

```go
inst := demoInstance{
    namespace: "default",
    name:      "demo-allinone",
    version:   "1.0.0",
    metadata:  map[string]string{"env": "dev"},
    endpoints: []registry.Endpoint{
        demoEndpoint{scheme: "grpc", address: "127.0.0.1:9000"},
    },
}

reg, _ := registry.Get()
reg.Register(context.Background(), inst)
```

### 3. Service discovery

- **Service name**: `demo-allinone` (monitoring itself)
- **Protocol Filtering**: Only listen to `grpc` and `http` protocols
- **Debounce**：200ms
- **AUTO-UPDATE**: Print endpoint list when instance changes

```go
res, _ := resolver.Get("default")

stateCh := make(chan resolver.State, 1)
res.AddWatch("demo-allinone", &mockClient{stateCh: stateCh})

go func() {
    for st := range stateCh {
        for _, ep := range st.GetEndpoints() {
            log.Printf("  - %s://%s", ep.GetProtocol(), ep.GetAddress())
        }
    }
}()
```

## Verification function

### View configuration

```bash
# View current configuration
etcdctl --endpoints=127.0.0.1:2379 get /demo/allinone/config

# Manually update configuration
etcdctl --endpoints=127.0.0.1:2379 put /demo/allinone/config "message: manually updated"
```

### View registration

```bash
# View all registered instances
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix

# View instances of a specific service
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry/default/demo-allinone --prefix
```

### View Lease

```bash
# View all leases
etcdctl --endpoints=127.0.0.1:2379 lease list

# View lease details
etcdctl --endpoints=127.0.0.1:2379 lease list --keys
```

## Configuration instructions

### Configure source configuration

```yaml
etcd:
  configSource:
    mode: blob
    key: /demo/allinone/config
    watch: true
    format: yaml
```

### Registration center configuration

```yaml
yggdrasil:
  registry:
    type: etcd
    config:
      prefix: /yggdrasil/registry
      ttl: 10s
      keepAlive: true
      retryInterval: 3s
```

### Service discovery configuration

```yaml
yggdrasil:
  resolver:
    default:
      type: etcd
      config:
        prefix: /yggdrasil/registry
        namespace: default
        protocols: ["grpc", "http"]
        debounce: 200ms
```

## Differences from other examples

| Example | Configuration Source | Registry | Service Discovery | Description |
|------|--------|---------|---------|------|
| allinone | ✅ Blob | ✅ | ✅ | Full example, all features |
| config-source/blob | ✅ Blob | ❌ | ❌ | Separate demo configuration source |
| config-source/kv | ✅ KV | ❌ | ❌ | Demo KV mode configuration source |
| registry | ❌ | ✅ | ❌ | Separate demo registration center |
| resolver | ❌ | ❌ | ✅ | Separate demo of service discovery |

## Learning path

1. **Step 1**: Run the [allinone](.) example to understand the overall process
2. **Step 2**: Learn [config-source/blob](../config-source/blob/) and master the usage of configuration source
3. **Step 3**: Learn [config-source/kv](../config-source/kv/) to understand KV mode
4. **Step 4**: Learn [registry](../registry/) and master the usage of the registration center
5. **Step 5**: Learn [resolver](../resolver/) and master service discovery usage

## FAQ

**Q: Why do you want to monitor your own service? **

A: This example is to demonstrate the service discovery function. In actual use, the resolver usually listens to other services.

**Q: Why is there a delay after the configuration is updated? **

A: Configuration updates need to go through etcd watch and application monitoring, and the delay is usually <100ms.

**Q: Will the instance be automatically renewed? **

A: Yes, automatic heartbeat renewal is enabled in the configuration `keepAlive: true`.

**Q: How to stop the sample? **

A: Press `Ctrl+C`, the example will exit gracefully and the instance will be unregistered automatically.

## Extension suggestions

1. **Multi-service deployment**: Start multiple allinone instances and observe the service discovery effect
2. **Load balancing integration**: Integrate resolver and balancer to achieve true load balancing
3. **gRPC Service**: Implement a real gRPC service instead of a simulated endpoint
4. **Configuration Encryption**: Use encrypted configuration to protect sensitive information
5. **Monitoring Alarms**: Add monitoring and alarms, monitor configuration, registration, and discovery status

## Related documents

- [etcd main documentation](../../../README.md)
- [Configuration Source Blob Mode](../config-source/blob/)
- [Configuration source KV mode](../config-source/kv/)
- [Registry Center](../registry/)
- [Service Discovery](../resolver/)
- [Sample overview](../README.md)
