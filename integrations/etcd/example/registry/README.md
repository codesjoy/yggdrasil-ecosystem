# Service registration center example

This example demonstrates how to use etcd as the registration center, register a service instance to etcd, and maintain heartbeats through the lease + keepalive mechanism to ensure that the instance is online.

## What you get

- Register the service instance to etcd
- Automatic heartbeat renewal to keep instances online
- Supports multi-protocol endpoints (e.g. gRPC, HTTP)
- Graceful offline: automatic de-registration when service stops
- Metadata management: record the region, zone, campus and other information of the instance

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
yggdrasil:
  registry:
    config:
      client:
        endpoints:
          - "your-etcd-endpoint:2379"
```

### 2. Run the example

```bash
cd integrations/etcd/example/registry
go run server.go
```

## Expected output

```
2024/01/26 10:00:00 [server] listening on 127.0.0.1:54321
2024/01/26 10:00:00 [registry] instance registered successfully
2024/01/26 10:00:00 [registry] service: default/example-registry-server/1.0.0
2024/01/26 10:00:00 [registry] endpoints: 2
2024/01/26 10:00:00   - grpc://127.0.0.1:54321
2024/01/26 10:00:00   - http://127.0.0.1:54321
2024/01/26 10:00:00 [server] running, press Ctrl+C to shutdown
2024/01/26 10:00:30 [registry] instance re-registered
...
```

## Verify registration

Use `etcdctl` to view registered instances:

```bash
# View all registered instances
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix

# View instances of a specific service
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry/default/example-registry-server --prefix

# Formatted output
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix -w json
```

Expected output (after formatting):
```json
{
  "namespace": "default",
  "name": "example-registry-server",
  "version": "1.0.0",
  "region": "us-west",
  "zone": "us-west-1",
  "campus": "campus-a",
  "metadata": {
    "env": "dev",
    "pod": "pod-1234567890",
    "started": "2024-01-26T10:00:00Z"
  },
  "endpoints": [
    {
      "scheme": "grpc",
      "address": "127.0.0.1:54321",
      "metadata": null
    },
    {
      "scheme": "http",
      "address": "127.0.0.1:54321",
      "metadata": null
    }
  ]
}
```

## Configuration instructions

### Key layout

The key layout of the instance in etcd:

```
<yggdrasil/registry>/<namespace>/<service>/<instanceKey(hash)>
```

Example:
```
/yggdrasil/registry/default/example-registry-server/a1b2c3d4e5f6...
```

### Registration center configuration

| Parameters | Description | Default value |
|------|------|--------|
| `client.endpoints` | etcd address list | `["127.0.0.1:2379"]` |
| `client.dialTimeout` | Connection timeout | `5s` |
| `prefix` | Registration prefix | `/yggdrasil/registry` |
| `ttl` | Lease TTL | `10s` |
| `keepAlive` | Whether to enable heartbeat renewal | `true` |
| `retryInterval` | Failure retry interval | `3s` |

###Instance interface

```go
type Instance interface {
    Namespace() string      // namespace
    Name() string          // Service name
    Version() string       // version number
    Region() string        // area
    Zone() string          // Availability Zone
    Campus() string        // park
    Metadata() map[string]string  // Metadata
    Endpoints() []Endpoint  // endpoint list
}
```

### Endpoint interface

```go
type Endpoint interface {
    Scheme() string              // Protocol: grpc, http, tcp, etc.
    Address() string             // Address: ip:port
    Metadata() map[string]string // Endpoint meta information
}
```

## Working principle

### Registration process

```
1. Create an Instance object
   ↓
2. Call reg.Register(context, inst)
   ↓
3. Generate instance key (based on namespace, name, version, endpoints)
   ↓
4. Create etcd lease (TTL)
   ↓
5. Write instance information to etcd (using lease)
   ↓
6. Start keepalive goroutine
   ↓
7. Send keepalive periodically to keep the lease valid.
```

### KeepAlive mechanism

- **Lease TTL**: Lease expiration time (default 10s)
- **KeepAlive interval**: automatically managed by etcd client (usually TTL/3)
- **Retry on failure**: If the keepalive fails, the lease will be re-created and registered.
- **Automatic recovery**: Automatically resume heartbeat after network recovery

### Anti-registration process

```
1. Stop signal received
   ↓
2. Call reg.Deregister(context, inst)
   ↓
3. Delete the instance key in etcd
   ↓
4. Close keepalive goroutine
   ↓
5. Exit gracefully
```

## Code structure description

```go
// 1. Initialize Yggdrasil framework
if err := yggdrasil.Init(appName); err != nil {
    log.Fatalf("yggdrasil.Init: %v", err)
}

// 2. Get the registration center
reg, err := registry.Get()
if err != nil {
    log.Fatalf("registry.Get: %v", err)
}

// 3. Create instance object
inst := demoInstance{
    namespace: "default",
    name:      appName,
    version:   "1.0.0",
    region:    "us-west",
    zone:      "us-west-1",
    campus:    "campus-a",
    metadata: map[string]string{
        "env": "dev",
        "pod": "pod-1234567890",
    },
    endpoints: []registry.Endpoint{
        demoEndpoint{scheme: "grpc", address: addr},
        demoEndpoint{scheme: "http", address: addr},
    },
}

// 4. Register an instance
if err := reg.Register(context.Background(), inst); err != nil {
    log.Fatalf("Register: %v", err)
}

// 5. Unregister the instance (gracefully offline)
if err := reg.Deregister(context.Background(), inst); err != nil {
    log.Printf("[registry] deregister failed: %v", err)
}
```

## Advanced usage

### Multiple instance registration

A service can register multiple instances (multiple network cards, multiple ports):

```go
inst := demoInstance{
    name: "my-service",
    endpoints: []registry.Endpoint{
        demoEndpoint{scheme: "grpc", address: "10.0.0.1:8080"},
        demoEndpoint{scheme: "grpc", address: "10.0.0.2:8080"},
        demoEndpoint{scheme: "http", address: "10.0.0.1:8081"},
    },
}
```

### Dynamically update meta information

Regularly update instance meta information:

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        inst.metadata["heartbeat"] = time.Now().Format(time.RFC3339)
        inst.metadata["load"] = fmt.Sprintf("%.2f", getLoadAverage())
        if err := reg.Register(context.Background(), inst); err != nil {
            log.Printf("re-register failed: %v", err)
        }
    }
}()
```

### Use namespace isolation

Different environments use different namespaces:

```go
// development environment
devInst := demoInstance{
    namespace: "dev",
    name:      "my-service",
    version:   "1.0.0",
}

// production environment
prodInst := demoInstance{
    namespace: "prod",
    name:      "my-service",
    version:   "1.0.0",
}
```

### Custom instance Key

By default, the instance key is generated based on the SHA1 hash of the instance content. You can also customize instance identification through metadata:

```go
inst := demoInstance{
    metadata: map[string]string{
        "instance_id": "my-custom-instance-id",
    },
}
```

## FAQ

**Q: Why did the instance disappear in etcd? **

A: Possible reasons:
1. keepAlive fails, check network connection and etcd status
2. The TTL setting is too short. It is recommended to set it to 10-30s.
3. The service process crashed and deregistration was not performed.

**Q: How to set a reasonable TTL? **

A: TTL selection suggestions:
- Development environment: 10-15s (quickly detect problems)
- Production environment: 30-60s (reduce network overhead)
- High availability requirements: 10-20s (fast failover)

**Q: How to distinguish between multiple instances? **

A: Each instance has a unique key (based on instance content hash):
- Different ports: generate different instances
- Different IP: generate different instances
- Different versions: generate different instances

**Q: How to monitor the registration status? **

A: It is recommended to monitor the following indicators:
1. Number of successful/failed registrations
2. keepAlive success/failure times
3. Changes in the number of instances
4. etcd connection status

**Q: Does it support TLS connection? **

A: Supported, add TLS certificate in configuration:
```yaml
client:
  tls:
    certFile: "/path/to/cert.pem"
    keyFile: "/path/to/key.pem"
    caFile: "/path/to/ca.pem"
```

## Best Practices

1. **Set TTL reasonably**: Choose the appropriate TTL according to business needs and balance performance and availability.
2. **Use namespaces**: Use different namespaces in different environments to avoid confusion.
3. **Record meta information**: Record useful information (version, deployment time, load, etc.) in metadata
4. **Graceful offline**: Make sure to perform de-registration before stopping the service
5. **Monitor registration status**: Monitor registration/de-registration and keepAlive status
6. **Use multi-protocol endpoints**: If the service supports multiple protocols, it is recommended to register them all

## Related documents

- [etcd main documentation](../../../README.md)
- [Service Discovery Example](../resolver/)
- [Blob pattern example](../config-source/blob/)
- [KV mode example](../config-source/kv/)
