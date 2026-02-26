# xDS control plane example

This example demonstrates how to implement a modern xDS control plane server that supports the Aggregated Discovery Service (ADS) protocol.

## Directory structure

```
control-plane/
├── main.go                 # Main entry and configuration loading
├── config.yaml             # Control plane configuration
├── xds-config.yaml         # xDS resource configuration
├── server/                 # xDS server implementation
│   └── server.go
├── snapshot/               # xDS snapshot build
│   ├── types.go
│   └── builder.go
├── watcher/                # File monitoring
│   └── file_watcher.go
└── README.md              # this document
```

## Features

- **Aggregated Discovery Service**: Supports unified discovery service of LDS, RDS, CDS, and EDS
- **Dynamic Configuration**: Supports hot update of configuration files without restarting the service
- **Modular Design**: Clear code structure, easy to maintain and expand
- **Load balancing strategy**: Support ROUND_ROBIN, RANDOM, LEAST_REQUEST, RING_HASH, MAGLEV
- **Circuit breaker configuration**: Support MaxConnections, MaxPendingRequests, MaxRequests, MaxRetries
- **Outlier Detection Configuration**: Full Outlier Detection support
- **Rate limiter configuration**: Configure Token Bucket rate limiting through Metadata
- **Graceful shutdown**: Supports SIGINT and SIGTERM signal processing
- **Version Control**: Atomic version number, supports incremental updates
- **Callback System**: Complete xDS Server callback for easy debugging and monitoring
- **Delta xDS**: Supports Delta streaming protocol

## Run steps

### 1. Start the control plane

```bash
cd integrations/xds/example/control-plane
go run .
```

The control plane will be started on port 18000.

### 2. Configuration instructions

**Control plane configuration** (config.yaml):

```yaml
server:
  port: 18000              # Control plane listening port
  nodeID: "yggdrasil.example.xds.control-plane"

xds:
  configFile: "xds-config.yaml"  # xDS resource configuration file
  watchInterval: 1s            # Configuration file monitoring interval

yggdrasil:
  logger:
    handler:
      default:
        type: "console"
        level: "info"
    writer:
      default:
        type: "console"
```

**xDS resource configuration** (xds-config.yaml):

```yaml
clusters:
  - name: "library-cluster"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"
    circuitBreakers:          # Circuit breaker configuration
      maxConnections: 10000
      maxPendingRequests: 1000
      maxRequests: 5000
      maxRetries: 3
    outlierDetection:          # Outlier detection configuration
      consecutive5xx: 5
      consecutiveGatewayFailure: 3
      consecutiveLocalOriginFailure: 2
      interval: "10s"
      baseEjectionTime: "30s"
      maxEjectionTime: "300s"
      maxEjectionPercent: 10
    rateLimiting:             # Current limiter configuration (via Metadata)
      maxTokens: 1000
      tokensPerFill: 100
      fillInterval: "1s"

endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555

listeners:
  - name: "library-service"
    address: "0.0.0.0"
    port: 10000
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "library-route"

routes:
  - name: "library-route"
    virtualHosts:
      - name: "library"
        domains: ["*"]
        routes:
          - match:
              path:
                prefix: "/"
            route:
              cluster: "library-cluster"
```

**Supported load balancing strategies**:
- `ROUND_ROBIN`: Polling
- `LEAST_REQUEST`: Minimum request
- `RANDOM`: random
- `RING_HASH`: consistent hashing
- `MAGLEV`: Maglev hash

## Working principle

### 1. Client connection process

```
client
↓ ADS Request
control plane
↓ Configure response
client
```

### 2. Resource type

| Type | Description | Example |
|------|------|------|
| LDS | Listener Discovery Service | library-service |
| RDS | Route Discovery Service | library-route |
| CDS | Cluster Discovery Service | library-cluster |
| EDS | Endpoint Discovery Service | 127.0.0.1:55555 |

### 3. Resource configuration relationship

```
Listener (library-service)
↓ Related
Route (library-route)
↓ Route to
Cluster (library-cluster)
↓ Contains
Endpoints (127.0.0.1:55555)
```

### 4. Configure hot update mechanism

```
Configuration file modification
↓ fsnotify monitoring
Anti-shake processing
↓ Load new configuration
BuildSnapshot
↓ Update cache
↓ Push new version
client
```

## Configuration Management

### Add new service

Modify the `xds-config.yaml` file:

```yaml
clusters:
  - name: "new-service"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"

endpoints:
  - clusterName: "new-service"
    endpoints:
      - address: "127.0.0.1"
        port: 8080

listeners:
  - name: "new-service-listener"
    address: "0.0.0.0"
    port: 10001
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "new-service-route"

routes:
  - name: "new-service-route"
    virtualHosts:
      - name: "new-service"
        domains: ["*"]
        routes:
          - match:
              path:
                prefix: "/"
            route:
              cluster: "new-service"
```

After the file is saved, the control plane automatically detects the changes and reloads the configuration.

### Modify load balancing policy

Modify `lbPolicy` in `xds-config.yaml`:

```yaml
clusters:
  - name: "library-cluster"
    lbPolicy: "LEAST_REQUEST"  # Change to least request strategy
```

### Route matching rules

Supports multiple route matching methods:

```yaml
routes:
  - name: "library-route"
    virtualHosts:
      - name: "library"
        domains: ["*"]
        routes:
          # prefix matching
          - match:
              path:
                prefix: "/api/"
            route:
              cluster: "library-cluster"
          # Exact path matching
          - match:
              path:
                path: "/exact/path"
            route:
              cluster: "library-cluster"
          # Header matching
          - match:
              path:
                prefix: "/"
              headers:
                - name: "x-version"
                  pattern: "exact"
                  value: "v2"
            route:
              cluster: "library-cluster-v2"
```

## Advanced features

### Circuit breaker configuration

```yaml
circuitBreakers:
  maxConnections: 10000        # Maximum number of connections
  maxPendingRequests: 1000      # Maximum number of pending requests
  maxRequests: 5000             # Maximum number of concurrent requests
  maxRetries: 3                # Maximum number of retries
```

### Outlier detection configuration

```yaml
outlierDetection:
  consecutive5xx: 5                       # Number of consecutive 5xx errors
  consecutiveGatewayFailure: 3            # Number of consecutive gateway failures
  consecutiveLocalOriginFailure: 2         # Number of consecutive local failures
  interval: "10s"                         # Detection interval
  baseEjectionTime: "30s"                  # Basic popup time
  maxEjectionTime: "300s"                   # Maximum pop-up time
  maxEjectionPercent: 10                   # Max popup percentage
  enforcingConsecutive5xx: 100              # Force continuous 5xx detection
  enforcingSuccessRate: 100                   # Forced success rate detection
  successRateMinimumHosts: 5                # Minimum number of hosts for success rate
  successRateRequestVolume: 100             # Success rate request volume
  successRateStdevFactor: 1900              # success rate standard deviation factor
  failurePercentageThreshold: 85            # Failure rate threshold
  failurePercentageMinimumHosts: 5          # Minimum number of hosts with failure rate
  failurePercentageRequestVolume: 50          # Failure rate request volumes
```

### Current limiter configuration

```yaml
rateLimiting:
  maxTokens: 1000        # Maximum number of tokens
  tokensPerFill: 100       # Number of tokens filled per time
  fillInterval: "1s"       # padding gap
```

## API interface

### StreamAggregatedResources

Implements the `AggregatedDiscoveryServiceServer` interface and supports the following callbacks:

- **OnStreamOpen**: triggered when the stream is opened
- **OnStreamClosed**: triggered when the stream is closed
- **OnStreamRequest**: Triggered when receiving a stream request
- **OnStreamResponse**: triggered when sending a stream response
- **OnFetchRequest**: Triggered when receiving a Fetch request
- **OnFetchResponse**: Triggered when sending Fetch response
- **OnDeltaStreamOpen**: triggered when the delta stream is opened
- **OnDeltaStreamClosed**: triggered when the delta stream is closed
- **OnStreamDeltaRequest**: Triggered when receiving a Delta request
- **OnStreamDeltaResponse**: Triggered when sending a Delta response

### Request/response format

**DiscoveryRequest**:
- Node: client node information
- TypeUrl: requested resource type
- ResourceNames: list of resource names
- VersionInfo: current version
- ResponseNonce: the nonce of the last response

**DiscoveryResponse**:
- VersionInfo: configuration version
- Nonce: respond to nonce
- TypeUrl: resource type
- Resources: Resource list (Any type)

## Integrate with other examples

### Basic integration example

Client configuration:

```yaml
xds:
  server:
    address: "127.0.0.1:18000"
  node:
    id: "basic-client"
```

### Traffic segmentation example

Configure multiple endpoints:

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
      - address: "127.0.0.2"
        port: 55555
```

## Debugging Tips

### View request log

```bash
[xDS Control Plane] Stream opened: id=0 type=ads
[xDS Control Plane] Stream request: id=0 node=basic-client resources=[library-service] version= type=type.googleapis.com/envoy.config.listener.v3.Listener
[xDS Control Plane] Stream response: id=0 version=1 type=type.googleapis.com/envoy.config.listener.v3.Listener resources=1
```

### Verify configuration

Check the configuration information in the log:

```bash
[xDS Control Plane] Building snapshot version=1 with 1 clusters, 1 endpoints, 1 listeners, 1 routes
[xDS Control Plane] Snapshot updated successfully: version=1
```

### Test connection

Test using grpcurl:

```bash
grpcurl -plaintext 127.0.0.1:18000 list
```

### Configure hot update test

After modifying the `xds-config.yaml` file, observe the log:

```bash
[File changed: /path/to/xds-config.yaml
[xDS Control Plane] Configuration file changed, reloading: /path/to/xds-config.yaml
[xDS Control Plane] Building snapshot version=2 with 1 clusters, 1 endpoints, 1 listeners, 1 routes
[xDS Control Plane] Snapshot updated successfully: version=2
```

## FAQ

### Q: How to add TLS support?

A: Add TLS credentials in `server/server.go`’s `NewServer` function:

```go
import "google.golang.org/grpc/credentials"

creds, err := credentials.NewServerTLSFromFile("cert.pem", "key.pem")
if err != nil {
    log.Fatal(err)
}

grpcServer := grpc.NewServer(
    grpc.Creds(creds),
    grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
)
```

### Q: How to implement dynamic configuration management interface?

A: You can add HTTP API or gRPC management interface to allow configuration modification at runtime:

```go
func (s *Server) UpdateConfig(newConfig *snapshot.XDSConfig) error {
    version := snapshotVersion.Add(1)
    versionStr := strconv.FormatUint(version, 10)
    builder := snapshot.NewBuilder(versionStr)
    snap, err := builder.BuildSnapshot(newConfig)
    if err != nil {
        return err
    }
    return s.cache.SetSnapshot(context.Background(), "", snap)
}
```

### Q: How does the control plane handle concurrent requests?

A: Use Envoy official `cache.SnapshotCache`, which already handles concurrent access and version consistency internally.

### Q: How to achieve configuration persistence?

A: Currently configured using YAML files, which can be expanded to:

1. **Database Storage**: Use SQLite, PostgreSQL and other databases
2. **Configuration Center**: Integrate Consul, Etcd, ZooKeeper
3. **GitOps**: Use Git to manage configuration versions

### Q: How to implement multi-tenant support?

A: Use Node ID as key in SnapshotCache:

```go
snapshotCache.SetSnapshot(context.Background(), nodeID, snap)
```

Different tenants use different Node IDs to connect and can obtain different configurations.

## Performance optimization suggestions

1. **Batch Update**: Multiple configuration changes are merged into one snapshot update
2. **Incremental update**: Use Delta xDS protocol to reduce network transmission
3. **Cache Warming**: Preload commonly used configurations at startup
4. **Connection pool**: Use connection pool to reuse gRPC connections
5. **Asynchronous processing**: Use independent goroutine to process file monitoring

## Monitoring and Observability

### Add indicator export

Can integrate Prometheus or OpenTelemetry:

```go
import "github.com/prometheus/client_golang"

var (
    requestCount = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "xds_requests_total",
            Help: "Total xDS requests",
        },
        []string{"type", "node_id"},
    )
)
```

### Log level

Configure log level:

```yaml
yggdrasil:
  logger:
    handler:
      default:
        type: "console"
        level: "debug"  # debug, info, warn, error
```

## Version compatibility

This implementation is based on Envoy v3 API and is compatible with the following versions:
- Envoy 1.18+
- go-control-plane v0.14.0+

## Further reading

- [Envoy xDS v3 API](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [go-control-plane](https://github.com/envoyproxy/go-control-plane)
- [xDS Protocol](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
