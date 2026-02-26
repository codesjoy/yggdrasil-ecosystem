# Traffic splitting example

This example demonstrates how to use xDS to implement traffic splitting and allocate traffic to different backend services based on weight.

## Directory structure

```
traffic-splitting/
├── client/
│   ├── main.go      # Client implementation, sending requests and counting traffic distribution
│   └── config.yaml  # Client xDS configuration
└── server/
    ├── main.go      # Server-side implementation, supporting multiple backends
    └── config.yaml  # Server configuration
```

## Features

- **Traffic Splitting**: Allocate traffic to different backends according to weight
- **Real-time Statistics**: The client counts the traffic distribution of each backend
- **Flexible Configuration**: Dynamically adjust weights through xDS configuration
- **Multi-backend support**: Supports two or more backend services

## Run steps

### 1. Start the xDS control plane

```bash
cd integrations/xds/example/control-plane
set XDS_CONFIG_FILE=traffic-splitting-xds-config.yaml
go run main.go --config config.yaml
```

### 2. Start backend service 1

```bash
cd integrations/xds/example/traffic-splitting/server
set BACKEND_ID=1
go run main.go --config config.yaml
```

### 3. Start backend service 2

Open a new terminal:

```bash
cd integrations/xds/example/traffic-splitting/server
set BACKEND_ID=2
set PORT=55556
go run main.go --config config.yaml
```

### 4. Run the client

```bash
cd integrations/xds/example/traffic-splitting/client
go run main.go --config config.yaml
```

## Expected output

The client will send 20 requests and count the traffic distribution of each backend:

```
2025/01/26 10:00:00 INFO Starting traffic splitting client...
2025/01/26 10:00:00 INFO Starting traffic splitting test...
2025/01/26 10:00:00 INFO GetShelf response - index: 0, name: shelves/1, theme: backend-1, server: backend-1
2025/01/26 10:00:00 INFO GetShelf response - index: 1, name: shelves/1, theme: backend-2, server: backend-2
2025/01/26 10:00:00 INFO GetShelf response - index: 2, name: shelves/1, theme: backend-1, server: backend-1
2025/01/26 10:00:00 INFO GetShelf response - index: 3, name: shelves/1, theme: backend-2, server: backend-2
...
2025/01/26 10:00:00 INFO Traffic splitting test completed - total_requests: 20
2025/01/26 10:00:00 INFO Traffic Distribution:
2025/01/26 10:00:00 INFO Backend - backend_id: backend-1, requests: 16, percentage: 80.00
2025/01/26 10:00:00 INFO Backend - backend_id: backend-2, requests: 4, percentage: 20.00
```

## Traffic segmentation scenario

### Scenario 1: A/B testing

- **Purpose**: Compare the performance of different versions
- **ratio**: 50% / 50%
- **Example**: Test new algorithm vs old algorithm

### Scenario 2: Grayscale release

- **Purpose**: Gradually roll out new features
- **ratio**: 90% / 10% → 80% / 20% → 50% / 50% → 0% / 100%
- **Example**: Grayscale release of new features

### Scenario 3: Capacity Management

- **Purpose**: Allocate traffic based on backend capacity
- **Scale**: Set according to actual capacity
- **Example**: High performance backend to handle more traffic

### Scenario 4: Multi-tenancy

- **Purpose**: Different tenants use different backends
- **ratio**: based on tenant needs
- **Example**: VIP tenant uses dedicated backend

## Configuration instructions

### xDS configuration (traffic-splitting-xds-config.yaml)

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 80      # Backend 1 weight 80%
      - address: "127.0.0.1"
        port: 55556
        weight: 20      # Backend 2 weight 20%
```

By modifying the weight, the traffic allocation ratio can be dynamically adjusted.

## Dynamically adjust traffic

### Adjust to 70/30

Modify `traffic-splitting-xds-config.yaml`:

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 70
      - address: "127.0.0.1"
        port: 55556
        weight: 30
```

The control plane automatically detects file changes and reloads the configuration, and clients immediately apply new traffic allocations.

### Adjust to 100/0 (completely switch to backend 1)

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 100
      - address: "127.0.0.1"
        port: 55556
        weight: 0
```

## Best Practices

### 1. Monitoring indicators

- **Traffic Distribution**: Monitor the actual traffic proportion of each backend in real time
- **Error Rate**: The error rate of each backend should not exceed the threshold
- **Response time**: P95, P99 delay of each backend
- **Resource Usage**: CPU, memory, network usage

### 2. Traffic switching strategy

- **Progressive Switching**: Gradually adjust weights in multiple stages
- **Monitoring Verification**: Verify system stability after each adjustment
- **Quick Rollback**: Immediately adjust back to the original weight if a problem is discovered

### 3. Backend health check

- **Active Health Check**: Check backend health status regularly
- **Fault Isolation**: Faulty backends are automatically removed from traffic distribution
- **Failure Recovery**: Healthy backends automatically rejoin traffic distribution

### 4. Weight calculation

- **Based on capacity**: weight ∝ capacity
- **Based on performance**: weight ∝ 1/response_time
- **Comprehensive evaluation**: weight = f(capacity, performance, cost)

## Differences from canary deployment

| Features | Canary Deployment | Traffic Splitting |
|------|-----------|----------|
| Purpose | Release new versions safely | Distribute traffic to different backends |
| Traffic proportion | Gradually increase | Fixed or dynamic adjustment |
| Backend version | Stable version + new version | Can be different versions |
| Rollback | Simple (switch back to 100%) | Simple (adjust weight) |
| Applicable scenarios | Version upgrade | A/B testing, capacity management, etc. |

## FAQ

Q: What is the difference between traffic splitting and canary deployment?
A: Canary deployment focuses on the security of version upgrades, while traffic splitting focuses on the flexibility of traffic distribution. Canary is a special application of traffic splitting.

Q: How to ensure accurate traffic distribution?
A: Using a weighted polling algorithm, it will be close to the theoretical ratio under a large number of requests. There will be some bias in small samples.

Q: Can it be split into 3 or more backends at the same time?
A: Yes, just add more endpoints and set weights in the xDS configuration.

Q: Does the sum of weights have to be 100?
A: No, the system will calculate the proportion of each weight to the total weight.

Q: How to dynamically adjust the traffic ratio?
A: Modify the weight in the xDS configuration file, the control plane will be automatically reloaded, and the client will take effect immediately.

Q: What happens if a backend goes down?
A: Healthy backends will redistribute traffic according to weight proportions, and faulty backends will be automatically eliminated.

Q: Will traffic splitting affect performance?
A: Almost no impact. Weight calculation and selection are O(1) complexity with minimal overhead.
