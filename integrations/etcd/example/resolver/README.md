#Service discovery example

This example demonstrates how to use etcd as a service discovery center, discover service instances from etcd, monitor instance changes in real time, and drive load balancing.

## What you get

- Discover all instances of the specified service from etcd
- Monitor instance changes in real time (add, delete, update)
- Automatically filter instances that do not match namespace and protocol
- Debounce mechanism to avoid jitter caused by frequent updates
- Real-time demonstration: register a new instance every 5 seconds and observe the service discovery effect

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
  resolver:
    default:
      config:
        client:
          endpoints:
            - "your-etcd-endpoint:2379"
```

### 2. Run the example

```bash
cd integrations/etcd/example/resolver
go run client.go
```

## Expected output

```
2024/01/26 10:00:00 [resolver] watching service: example-registry-server
2024/01/26 10:00:00 [client] running, press Ctrl+C to exit
2024/01/26 10:00:05 [registry] registered instance 1 (grpc://127.0.0.1:9001)
2024/01/26 10:00:05 [resolver] state updated
2024/01/26 10:00:05   service: example-registry-server
2024/01/26 10:00:05   namespace: default
2024/01/26 10:00:05   revision: 123
2024/01/26 10:00:05   endpoints: 2
2024/01/26 10:00:05     - grpc://127.0.0.1:9001
2024/01/26 10:00:05       version: 1.0.0
2024/01/26 10:00:05       region: us-west
2024/01/26 10:00:05       zone: us-west-1
2024/01/26 10:00:05     - http://127.0.0.1:8081
2024/01/26 10:00:10 [registry] registered instance 2 (grpc://127.0.0.1:9002)
2024/01/26 10:00:10 [resolver] state updated
2024/01/26 10:00:10   endpoints: 4
...
```

## Manually add instances

You can also manually use `etcdctl` to add instances and observe the service discovery effect:

```bash
# View current instance
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry --prefix

# Add instances manually
etcdctl put /yggdrasil/registry/default/example-registry-server/abc123 '{
  "namespace": "default",
  "name": "example-registry-server",
  "version": "1.0.0",
  "endpoints": [
    {
      "scheme": "grpc",
      "address": "192.168.1.100:8080"
    }
  ]
}'

# Delete instance
etcdctl del /yggdrasil/registry/default/example-registry-server/abc123
```

## Configuration instructions

### How service discovery works

```
1. Add Watch
   ↓
2. Watch etcd prefix (/yggdrasil/registry/<namespace>/<service>)
   ↓
3. Receive change event
   ↓
4. Debounce (anti-shake)
   ↓
5. Pull the current instance list in full
   ↓
6. Filter unmatched instances (namespace, protocol)
   ↓
7. Update State
   ↓
8. Notify all Watchers
```

### Resolver configuration

| Parameters | Description | Default value |
|------|------|--------|
| `client.endpoints` | etcd address list | `["127.0.0.1:2379"]` |
| `client.dialTimeout` | Connection timeout | `5s` |
| `prefix` | Registration prefix | `/yggdrasil/registry` |
| `namespace` | namespace | `default` |
| `protocols` | Supported protocol list | `["grpc", "http"]` |
| `debounce` | Anti-shake delay | `200ms` |

### Filter rules

Resolver automatically filters the following instances:

1. **Namespace mismatch**: The namespace of the instance is inconsistent with the configured namespace
2. **Protocol mismatch**: The endpoint protocol of the instance is not in the configured protocols list
3. **Service name mismatch**: The service name of the instance is inconsistent with the service name of the watch

### State structure

```go
type State interface {
    GetEndpoints() []Endpoint  // Get all endpoints
    GetAttributes() map[string]any  // Get status attributes
}

type Endpoint interface {
    Name() string              // endpoint name
    Protocol() string         // protocol
    Address() string         // address
    GetAttributes() map[string]any  // Endpoint properties
}
```

## Code structure description

```go
// 1. Initialize Yggdrasil framework
if err := yggdrasil.Init(appName); err != nil {
    log.Fatalf("yggdrasil.Init: %v", err)
}

// 2. Get Resolver
res, err := resolver.Get("default")
if err != nil {
    log.Fatalf("resolver.Get: %v", err)
}

// 3. Create Watcher
stateCh := make(chan resolver.State, 10)
watcher := &mockClient{stateCh: stateCh}

// 4. Add Watch
serviceName := "example-registry-server"
if err := res.AddWatch(serviceName, watcher); err != nil {
    log.Fatalf("AddWatch: %v", err)
}

// 5. Implement the Watcher interface
type mockClient struct {
    stateCh chan resolver.State
}

func (m *mockClient) UpdateState(st resolver.State) {
    select {
    case m.stateCh <- st:
    default:
        // Channel is full, discard updates
    }
}

// 6. Handle State updates
go func() {
    for st := range stateCh {
        eps := st.GetEndpoints()
        for _, ep := range eps {
            log.Printf("discovered: %s://%s", ep.GetProtocol(), ep.GetAddress())
        }
    }
}()

// 7. Stop Watch
res.DelWatch(serviceName, watcher)
```

## Advanced usage

### Multi-service Watch

Monitor multiple services at the same time:

```go
services := []string{"service-a", "service-b", "service-c"}
for _, svc := range services {
    watcher := &mockClient{stateCh: make(chan resolver.State, 10)}
    if err := res.AddWatch(svc, watcher); err != nil {
        log.Printf("add watch failed for %s: %v", svc, err)
    }
}
```

### Protocol filtering

Discover only endpoints for specific protocols:

```yaml
yggdrasil:
  resolver:
    default:
      config:
        protocols:
          - grpc  # Only endpoints for the grpc protocol are discovered
```

### Namespace isolation

Different environments use different namespaces:

```yaml
yggdrasil:
  resolver:
    dev:
      config:
        namespace: dev  # development environment
    prod:
      config:
        namespace: prod  # production environment
```

### Debounce adjustments

Adjust the anti-shake delay according to business needs:

```yaml
yggdrasil:
  resolver:
    default:
      config:
        debounce: 500ms  # Greater anti-shake delay, reduced update frequency
```

### Integrate with load balancing

Endpoints discovered by the Resolver can be used directly for load balancing:

```go
import "github.com/codesjoy/yggdrasil/v2/balancer"

bal := balancer.Get("round_robin")
if err := res.AddWatch("my-service", bal); err != nil {
    log.Fatalf("add watch failed: %v", err)
}

// Select endpoints using a load balancer
endpoint, err := bal.Pick(context.Background())
if err != nil {
    log.Printf("pick failed: %v", err)
    return
}
log.Printf("selected: %s://%s", endpoint.Protocol(), endpoint.Address())
```

## FAQ

**Q: Why no instances found? **

A: Check the following points:
1. Confirm that the service has been registered to etcd (use `etcdctl get /yggdrasil/registry --prefix` to view)
2. Confirm that the namespace is configured correctly
3. Confirm that the protocol configuration is correct
4. Confirm that the service name is correct

**Q: Didn’t receive notification after instance update? **

A: Possible reasons:
1. There is no change in the instance key (the same content will not trigger an update)
2. Debounce delay causes notification delay
3. The watcher channel is full and updates are discarded.

**Q: How to control the update frequency? **

A: Adjust debounce parameters:
- Development environment: 100-200ms (fast response)
- Production environment: 300-500ms (reduce jitter)

**Q: Does it support multiple Resolvers? **

A: Yes, you can use different Resolvers for different services:
```yaml
yggdrasil:
  resolver:
    default:
      type: etcd
      config:
        namespace: default
    special:
      type: etcd
      config:
        namespace: special
```

**Q: How to view the currently discovered instances? **

A: Use `etcdctl` to view:
```bash
etcdctl --endpoints=127.0.0.1:2379 get /yggdrasil/registry/default/my-service --prefix
```

## Best Practices

1. **Set Debounce reasonably**: Choose the appropriate anti-shake delay according to business needs
2. **Use Namespace isolation**: Use different namespaces in different environments
3. **Restrict Protocol**: Only monitor the required protocols to reduce unnecessary instance delivery
4. **Handling Channel Full Load**: The watcher channel should have enough buffering or handle the full load situation correctly.
5. **Monitoring discovery status**: monitor the number of instances, update frequency and other indicators
6. **Graceful Stop**: Call `DelWatch` to stop listening when the application stops.

## Related documents

- [etcd main documentation](../../../README.md)
- [Registration Center Example](../registry/)
- [Blob pattern example](../config-source/blob/)
- [KV mode example](../config-source/kv/)
