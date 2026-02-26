# Load balancing example

This example demonstrates how to use xDS to implement different load balancing strategies, including round robin, random, and least requests.

## Directory structure

```
load-balancing/
├── client/
│   ├── main.go      # Client implementation, sending requests and counting traffic distribution
│   └── config.yaml  # Client xDS configuration
└── server/
    ├── main.go      # Server-side implementation, multiple instances
    └── config.yaml  # Server configuration
```

## Features

- **Multi-instance support**: Start multiple service instances to test load balancing
- **Traffic Statistics**: The client counts the request distribution of each service instance
- **Multiple strategies**: Supports round_robin, random, least_request and other strategies
- **Real-time monitoring**: Output request statistics for each service instance

## Run steps

### 1. Start the xDS control plane

```bash
cd integrations/xds/example/control-plane
set XDS_CONFIG_FILE=load-balancing-xds-config.yaml
go run main.go --config config.yaml
```

### 2. Start multiple service instances

Open multiple terminal windows and start different service instances:

```bash
# Terminal 1 - Service Instance 1
cd integrations/xds/example/load-balancing/server
set SERVER_ID=1
go run main.go --config config.yaml

# Terminal 2 - Service Instance 2
cd integrations/xds/example/load-balancing/server
set SERVER_ID=2
set PORT=55556
go run main.go --config config.yaml

# Terminal 3 - Service Instance 3
cd integrations/xds/example/load-balancing/server
set SERVER_ID=3
set PORT=55557
go run main.go --config config.yaml
```

### 3. Run the client

```bash
cd integrations/xds/example/load-balancing/client
go run main.go --config config.yaml
```

## Expected output

The client will send 30 requests and count the traffic distribution of each service instance:

### Round Robin strategy

```
2025/01/26 10:00:00 INFO Starting load balancing client...
2025/01/26 10:00:00 INFO Starting load balancing test...
2025/01/26 10:00:00 INFO GetShelf response - index: 0, name: shelves/1, theme: server-1, server: server-1
2025/01/26 10:00:00 INFO GetShelf response - index: 1, name: shelves/1, theme: server-2, server: server-2
2025/01/26 10:00:00 INFO GetShelf response - index: 2, name: shelves/1, theme: server-3, server: server-3
2025/01/26 10:00:00 INFO GetShelf response - index: 3, name: shelves/1, theme: server-1, server: server-1
...
2025/01/26 10:00:00 INFO Load balancing test completed - total_requests: 30
2025/01/26 10:00:00 INFO Traffic Distribution:
2025/01/26 10:00:00 INFO Server - server_id: server-1, requests: 10, percentage: 33.333
2025/01/26 10:00:00 INFO Server - server_id: server-2, requests: 10, percentage: 33.333
2025/01/26 10:00:00 INFO Server - server_id: server-3, requests: 10, percentage: 33.333
```

## Load balancing strategy

### 1. Round Robin

**Features**:
- Distribute requests in turn in order
-Each instance gets roughly equal traffic
- Does not take into account the current load of the instance

**Applicable scenarios**:
- All instances perform similarly
- Request processing times are similar
- Need to distribute traffic fairly

**Implementation principle**:
```go
func (lb *roundRobinBalancer) Pick() *endpoint {
    index := lb.current
    lb.current = (lb.current + 1) % len(lb.endpoints)
    return lb.endpoints[index]
}
```

### 2. Random (random)

**Features**:
- Randomly select an instance
-Tends to be evenly distributed under a large number of requests
- Simple to implement

**Applicable scenarios**:
- Larger number of instances
- No strict guarantee of uniform distribution is required
- Low requirements for fairness

**Implementation principle**:
```go
func (lb *randomBalancer) Pick() *endpoint {
    index := rand.Intn(len(lb.endpoints))
    return lb.endpoints[index]
}
```

### 3. Least Request (least request)

**Features**:
- Select the instance with the fewest current requests
- Dynamically adapt to instance load
- Suitable for scenarios with large differences in processing time

**Applicable scenarios**:
- Request processing time varies greatly
- Some instances have strong performance
- Requires dynamic load balancing

**Implementation principle**:
```go
func (lb *leastRequestBalancer) Pick() *endpoint {
    var selected *endpoint
    minRequests := int64(^uint64(0) >> 1)

    for _, ep := range lb.endpoints {
        if ep.activeRequests < minRequests {
            minRequests = ep.activeRequests
            selected = ep
        }
    }

    return selected
}
```

## Technical points

### 1. Service instance identification

The server identifies the instance ID through environment variables:

```go
serverID := os.Getenv("SERVER_ID")
if serverID == "" {
    serverID = "1"
}
```

### 2. Metadata transfer

The server adds the instance ID to the response:

```go
func (s *LibraryImpl) GetShelf(
    ctx context.Context,
    req *librarypb2.GetShelfRequest,
) (*librarypb2.Shelf, error) {
    _ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
    _ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
    return &librarypb2.Shelf{
        Name:  req.Name,
        Theme: "server-" + s.serverID,
    }, nil
}
```

### 3. Traffic statistics

The client counts the number of requests for each instance:

```go
serverCounts := make(map[string]int)
var mu sync.Mutex

for i := 0; i < requestCount; i++ {
    // ... Send request ...

    mu.Lock()
    serverCounts[serverID]++
    mu.Unlock()
}
```

## Configuration instructions

### xDS configuration (load-balancing-xds-config.yaml)

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 1
      - address: "127.0.0.1"
        port: 55556
        weight: 1
      - address: "127.0.0.1"
        port: 55557
        weight: 1
```

The three instances have the same weight and the traffic will be evenly distributed.

## Test different load balancing strategies

### Test Round Robin

Modify control-plane/load-balancing-xds-config.yaml:

```yaml
lbPolicy: "ROUND_ROBIN"
```

Run the client and observe whether the requests are evenly distributed (33.33% per instance).

### Test Random

Modify control-plane/load-balancing-xds-config.yaml:

```yaml
lbPolicy: "RANDOM"
```

Run the client multiple times and observe whether the distribution is roughly even each time.

### Test Least Request

Modify control-plane/load-balancing-xds-config.yaml:

```yaml
lbPolicy: "LEAST_REQUEST"
```

You need to simulate load differences on different instances to see the effect.

## Performance comparison

| Strategy | Complexity | Uniformity | Adaptability | Overhead |
|------|--------|--------|--------|------|
| Round Robin | O(1) | High | Low | Low |
| Random | O(1) | Medium | Low | Very Low |
| Least Request | O(n) | High | High | Medium |

## FAQ

Q: How to choose the appropriate load balancing strategy?
A: Choose according to the business scenario:
- Similar request processing times → Round Robin
- Large number of instances with similar performance → Random
- Large difference in request processing time → Least Request

Q: How does the Least Request policy track the number of requests?
A: Each endpoint maintains an activeRequests counter, which is +1 when the request starts and -1 when it ends.

Q: Can the load balancing strategy be dynamically switched?
A: Yes, by updating the lb_policy field of the xDS configuration, the client will automatically apply the new policy.

Q: Is the load balancing strategy implemented on the client side or the server side?
A: Implemented on the client side, xDS only provides endpoint lists and policy configurations, and the actual load balancing is performed by the client.

Q: How to verify whether load balancing is effective?
A: Use the traffic distribution statistics output by the client to observe whether requests are distributed to each instance as expected.

Q: What should I do if an instance goes down?
A: The xDS control plane will remove the faulty instance from the endpoint list, and the client will automatically distribute traffic to other healthy instances.
