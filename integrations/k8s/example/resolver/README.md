# Kubernetes service discovery example

This example demonstrates how to use k8s as a service discovery center to discover service instances from Kubernetes, monitor instance changes in real time, and drive load balancing.

## What you get

- Discover all instances of a specified service from Kubernetes
- Monitor instance changes in real time (add, delete, update)
- Supports Endpoints and EndpointSlice modes
- Automatically filter instances that do not match namespace and protocol
- Endpoint meta information management (nodeName, zone, etc.)
- Real-time demonstration of service discovery effect

## Preconditions

1. Have an accessible Kubernetes cluster
2. The services to be discovered have been deployed in the cluster
3. Go 1.19+ installed locally
4. kubectl has configured cluster access permissions

### Quickly start local Kubernetes

If you don't have a Kubernetes cluster, you can use one of the following tools to get started quickly:

**Using Minikube:**
```bash
minikube start
```

**Use Kind:**
```bash
kind create cluster
```

**Using Docker Desktop:**
```bash
# Docker Desktop has Kubernetes built-in, just enable it in the settings
```

Verify that the Kubernetes cluster is functioning properly:
```bash
kubectl cluster-info
kubectl get nodes
```

## Startup method

### 1. Deploy test service

First deploy a test service in Kubernetes:

```bash
# Create deployment
kubectl create deployment nginx --image=nginx:latest -n default

# Create service
kubectl expose deployment nginx --port=80 --target-port=80 -n default

# View services
kubectl get svc nginx -n default
```

Or use a YAML file:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: default
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
```

```bash
kubectl apply -f service.yaml
```

### 2. Verification service

View the service’s Endpoints:
```bash
kubectl get endpoints nginx -n default -o yaml
```

View the EndpointSlice of the service:
```bash
kubectl get endpointslices -n default -l kubernetes.io/service-name=nginx -o yaml
```

### 3. Modify configuration (optional)

If you need to modify the configuration, please edit [config.yaml](config.yaml):

```yaml
yggdrasil:
  resolver:
    k8s-resolver:
      type: kubernetes
      config:
        namespace: default          # Kubernetes namespace
        mode: endpointslice         # Mode: endpoints or endpointslice
        portName: http            # Port name (optional)
        port: 0                   # Port number (optional, alternative to portName)
        protocol: http             # protocol type
        kubeconfig: ""             # kubeconfig file path (empty means using in-cluster config)
        resyncPeriod: 30s         # Resync cycle
        timeout: 10s               # timeout
        backoff:
          baseDelay: 1s           # Basic backoff delay
          multiplier: 1.6          # Backoff multiplier
          jitter: 0.2              # Jitter coefficient
          maxDelay: 30s           # Maximum backoff delay
```

### 4. Run the example

```bash
cd integrations/k8s/example/resolver
go run main.go
```

## Expected output

```
INFO client created, press Ctrl+C to exit...
INFO discovered endpoint: http://10.244.0.5:80, protocol=http, nodeName=minikube, zone=
INFO discovered endpoint: http://10.244.0.6:80, protocol=http, nodeName=minikube, zone=
INFO discovered endpoint: http://10.244.0.7:80, protocol=http, nodeName=minikube, zone=
```

## Test service discovery

### Expansion service

```bash
kubectl scale deployment nginx --replicas=5 -n default
```

Observe the program output and you should be able to see the new endpoint.

### Reduction service

```bash
kubectl scale deployment nginx --replicas=2 -n default
```

Watch the program output and you should see the endpoints reduced.

### Update service

```bash
kubectl set image deployment/nginx nginx=nginx:1.25 -n default
```

Watch the program output, the endpoint will be updated (IP may change).

### Delete service

```bash
kubectl delete service nginx -n default
```

Watch the program output and you should see all endpoints deleted.

## Configuration instructions

### ResolverConfig parameters

| Parameters | Type | Description | Default value |
|------|------|------|--------|
| `Namespace` | string | Kubernetes namespace | From environment variable `KUBERNETES_NAMESPACE` or `default` |
| `Mode` | string | Pattern: `endpoints` or `endpointslice` | `endpointslice` |
| `PortName` | string | port name | empty (use first port) |
| `Port` | int32 | Port number | 0 (selected based on PortName) |
| `Protocol` | string | protocol type | `grpc` |
| `Kubeconfig` | string | kubeconfig file path | empty (use in-cluster config) |
| `ResyncPeriod` | time.Duration | Resynchronization period | 0 (no resynchronization) |
| `Timeout` | time.Duration | Timeout | 0 (no timeout) |
| `Backoff` | backoffConfig | Backoff configuration | See below |

### Backoff configuration

| Parameters | Type | Description | Default value |
|------|------|------|--------|
| `BaseDelay` | time.Duration | Basic backoff delay | `1s` |
| `Multiplier` | float64 | Backoff multiple | `1.6` |
| `Jitter` | float64 | Jitter coefficient (0-1) | `0.2` |
| `MaxDelay` | time.Duration | Maximum backoff delay | `30s` |

### Mode description

**Endpoints mode:**
```yaml
mode: endpoints
```
- Using the Kubernetes `Endpoints` API
- Suitable for versions prior to Kubernetes 1.17
- Does not support large-scale clusters (Endpoints have size limits)

**EndpointSlice mode (recommended):**
```yaml
mode: endpointslice
```
- Using the Kubernetes `EndpointSlice` API
- Suitable for Kubernetes 1.17+ version
- Supports large-scale clustering (up to 100 endpoints per EndpointSlice)
-Support topology-aware routing (zone-aware)

### Working principle

```
1. Add Watch
   ↓
2. Watch Kubernetes Endpoints/EndpointSlice
   ↓
3. Receive change event
   ↓
4. Pull the current endpoint list in full
   ↓
5. Filter unmatched endpoints (namespace, port, protocol)
   ↓
6. Convert to Yggdrasil State
   ↓
7. Notify all Watchers
```

### Endpoint properties

Each endpoint contains the following properties:

| Properties | Description | Source |
|------|------|------|
| `address` | Endpoint address (IP:Port) | Endpoints/EndpointSlice |
| `protocol` | Protocol type (grpc/http) | Configuration |
| `nodeName` | Node name | TargetRef |
| `zone` | Availability Zone | Zone |
| `podName` | Pod name | TargetRef |

### Code structure description

```go
// 1. Initialize Yggdrasil framework
if err := yggdrasil.Init("k8s-resolver-example"); err != nil {
    panic(err)
}

// 2. Get Client
cli, err := yggdrasil.NewClient("downstream-service")
if err != nil {
    panic(err)
}
defer cli.Close()

// 3. Create a custom Watcher
watcher := &stateLogger{}

// 4. Add Watch
res, err := resolver.Get("k8s-resolver")
if err != nil {
    panic(err)
}
if err := res.AddWatch("nginx", watcher); err != nil {
    panic(err)
}

// 5. Implement the Watcher interface
type stateLogger struct{}

func (sl *stateLogger) UpdateState(st resolver.State) {
    for _, ep := range st.GetEndpoints() {
        attrs := ep.GetAttributes()
        slog.Info("discovered endpoint",
            "address", ep.GetAddress(),
            "protocol", ep.GetProtocol(),
            "nodeName", attrs["nodeName"],
            "zone", attrs["zone"],
        )
    }
}

// 6. Wait for signal
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
<-sig

// 7. Clean up resources
yggdrasil.Stop()
```

## Advanced usage

### Multi-service Watch

Monitor multiple services at the same time:

```go
services := []string{"service-a", "service-b", "service-c"}
for _, svc := range services {
    watcher := &stateLogger{}
    if err := res.AddWatch(svc, watcher); err != nil {
        slog.Error("add watch failed", "service", svc, "error", err)
    }
}
```

###Specify port

**Use port name (recommended):**
```yaml
portName: grpc
```

**Port number used:**
```yaml
port: 9090
```

### Namespace isolation

Different environments use different namespaces:

```yaml
yggdrasil:
  resolver:
    dev-resolver:
      config:
        namespace: dev  # development environment
    prod-resolver:
      config:
        namespace: prod  # production environment
```

### Resync

Periodically resynchronize the endpoint list:

```yaml
resyncPeriod: 30s  # Resync every 30 seconds
```

### Timeout control

Set operation timeout:

```yaml
timeout: 10s  # 10 seconds timeout
```

### Integrate with load balancing

Endpoints discovered by the Resolver can be used directly for load balancing:

```go
import "github.com/codesjoy/yggdrasil/v2/balancer"

// Get load balancer
bal := balancer.Get("round_robin")

// Register Resolver as Balancer's Watcher
res, _ := resolver.Get("k8s-resolver")
res.AddWatch("nginx", bal)

// Select endpoints using a load balancer
endpoint, err := bal.Pick(context.Background())
if err != nil {
    slog.Error("pick failed", "error", err)
    return
}
slog.Info("selected", "endpoint", endpoint.Address())
```

### Topology aware routing

EndpointSlice mode supports zone-aware routing:

```go
// Get zone information from endpoint properties
for _, ep := range st.GetEndpoints() {
    attrs := ep.GetAttributes()
    zone := attrs["zone"]
    nodeName := attrs["nodeName"]

    // Select the optimal endpoint based on zone
    if zone == currentZone {
        // Prefer endpoints in the same zone
    }
}
```

## RBAC permissions

Programs require the following RBAC permissions to access Endpoints/EndpointSlice:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: endpoints-reader
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: endpoints-reader
  namespace: default
subjects:
- kind: ServiceAccount
  name: your-serviceaccount
roleRef:
  kind: Role
  name: endpoints-reader
  apiGroup: rbac.authorization.k8s.io
```

## FAQ

**Q: Why is no endpoint found? **

A: Check the following points:
1. Confirm that the service exists: `kubectl get svc nginx -n default`
2. Confirm that the service has Endpoints: `kubectl get endpoints nginx -n default`
3. Confirm that the namespace is configured correctly
4. Confirm that the port configuration is correct
5. Check RBAC permissions

**Q: What is the difference between Endpoints and EndpointSlice modes? **

A: Main differences:
- Endpoints: Old version API, not suitable for large-scale clusters
- EndpointSlice: New version of API to support large-scale clustering and topology-aware routing
- EndpointSlice mode is recommended (Kubernetes 1.17+)

**Q: How to choose the port? **

A: There are two ways:
1. Use `portName` (recommended): select based on port name
2. Use `port`: select based on port number
- If neither is specified, the first port is used

**Q: Didn’t receive notification after endpoint update? **

A: Possible reasons:
1. There are no changes to the service (the same content will not trigger updates)
2. Pod is not running properly: `kubectl get pods -n default`
3. Kubernetes cluster network issues
4. Watcher channel is full and updates are discarded

**Q: How to view the currently discovered endpoints? **

A: Use kubectl to view:
```bash
kubectl get endpoints nginx -n default -o yaml
kubectl get endpointslices -n default -l kubernetes.io/service-name=nginx -o yaml
```

**Q: Does it support multiple Resolvers? **

A: Yes, you can use different Resolvers for different services:
```yaml
yggdrasil:
  resolver:
    default:
      type: kubernetes
      config:
        namespace: default
    special:
      type: kubernetes
      config:
        namespace: special
```

**Q: How to use different kubeconfig files? **

A: Set `Kubeconfig` parameters:
```yaml
config:
  kubeconfig: "/path/to/kubeconfig.yaml"
```

## Best Practices

1. **Use EndpointSlice mode**: It is recommended to use EndpointSlice mode (Kubernetes 1.17+)
2. **Use port name**: It is recommended to use `portName` instead of `port`, which is more semantic
3. **Namespace isolation**: use different namespaces in different environments
4. **Properly set ResyncPeriod**: Resynchronize regularly to ensure data consistency
5. **Set timeout**: Set a reasonable timeout according to business needs
6. **Monitoring Discovery Status**: Monitor the number of endpoints, update frequency and other indicators
7. **Topology awareness**: Use the zone information of EndpointSlice to implement topology-aware routing
8. **RBAC Permissions**: Configure RBAC using the principle of least privilege
9. **Graceful Stop**: Call `DelWatch` to stop listening when the application stops
10. **Logging**: Record endpoint change logs to facilitate auditing and troubleshooting.

## Related documents

- [k8s main documentation](../../README.md)
- [ConfigMap configuration source example](../config-source/)
- [Secret configuration source example](../secret-source/)
- [yggdrasil resolver documentation](../../../../docs/resolver.md)

## quit

Press `Ctrl+C` to exit the program.
