# Canary deployment example

This example demonstrates how to use xDS to implement a progressive canary deployment to safely and gradually switch traffic from a stable version to a new version.

## Directory structure

```
canary/
├── client/
│   ├── main.go      # Client implementation, simulating progressive traffic switching
│   └── config.yaml  # Client xDS configuration
└── server/
    ├── main.go      # Server-side implementation, supports stable version and canary version
    └── config.yaml  # Server configuration
```

## Features

- **Progressive traffic switching**: gradually increase from 5% to 100%
- **Multi-stage verification**: Each stage has a clear traffic ratio and verification time
- **Real-time monitoring**: Output traffic distribution statistics after each stage is completed
- **Safe Rollback**: You can roll back to a stable version at any time

## Run steps

### 1. Start the xDS control plane

```bash
cd integrations/xds/example/control-plane
set XDS_CONFIG_FILE=canary-xds-config.yaml
go run main.go --config config.yaml
```

### 2. Start the stable version service

```bash
cd integrations/xds/example/canary/server
set DEPLOYMENT_TYPE=stable
go run main.go --config config.yaml
```

### 3. Start the canary version service

Open a new terminal:

```bash
cd integrations/xds/example/canary/server
set DEPLOYMENT_TYPE=canary
set PORT=55556
go run main.go --config config.yaml
```

### 4. Run the client

```bash
cd integrations/xds/example/canary/client
go run main.go --config config.yaml
```

## Expected output

The client will simulate 6 stages of progressive traffic switching:

```
2025/01/26 10:00:00 INFO Starting canary deployment client...
2025/01/26 10:00:00 INFO Starting canary deployment test with progressive traffic increase...
2025/01/26 10:00:00 INFO Starting stage - stage_name: Stage 1: 5% to stable, canary_percentage: 5
2025/01/26 10:00:00 INFO Stage completed - stage_name: Stage 1: 5% to stable, expected_canary_percentage: 5, actual_canary_percentage: 5.20, total_requests: 100, stable_count: 95, canary_count: 5
2025/01/26 10:00:00 INFO Waiting before next stage... - seconds: 5
2025/01/26 10:00:05 INFO Starting stage - stage_name: Stage 2: 10% to stable, canary_percentage: 10
2025/01/26 10:00:05 INFO Stage completed - stage_name: Stage 2: 10% to stable, expected_canary_percentage: 10, actual_canary_percentage: 9.80, total_requests: 100, stable_count: 90, canary_count: 10
...
2025/01/26 10:00:25 INFO Stage completed - stage_name: Stage 6: 100% to stable, expected_canary_percentage: 100, actual_canary_percentage: 100.00, total_requests: 100, stable_count: 0, canary_count: 100
2025/01/26 10:00:25 INFO Canary deployment test completed successfully
```

## Canary deployment strategy

### Phase 1: Initial Verification (5%)
- **Purpose**: Verify that the basic functions of the canary version are normal
- **Duration**: 5-10 minutes
- **Focus on indicators**: error rate, response time

### Phase 2: Small-scale expansion (10%)
- **Purpose**: Expand the scope of verification and discover potential problems
- **Duration**: 10-15 minutes
- **Focus on indicators**: system resource usage, database connection

### Stage 3: Medium scale (25%)
- **Purpose**: Verify performance under moderate load
- **Duration**: 15-20 minutes
- **Focus on indicators**: concurrent processing capabilities, cache hit rate

### Phase 4: Large-Scale Verification (50%)
- **Purpose**: Verify stability under high load
- **Duration**: 20-30 minutes
- **Focus on indicators**: system bottlenecks, dependent service pressure

### Stage 5: Nearing completion (75%)
- **Purpose**: Prepare for full switchover
- **Duration**: 15-20 minutes
- **Focus on indicators**: All key indicators

### Stage 6: Full Switch (100%)
- **Purpose**: Complete canary deployment
- **Duration**: Continuous monitoring
- **Indicators to watch**: Long-term stability

## Configuration instructions

### xDS configuration (canary-xds-config.yaml)

The control plane configuration uses `canary-xds-config.yaml`, which defines canary weights:

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 95      # Stable version weight 95%
      - address: "127.0.0.1"
        port: 55556
        weight: 5       # The canary version has a weight of 5%
```

By modifying the weight ratio, traffic allocation can be dynamically adjusted.

## Best Practices

### 1. Monitoring indicators

Key indicators include:
- **Error rate**: Canary version should not be higher than stable version
- **Response Time**: P95 and P99 latency should not increase significantly
- **Resource Usage**: CPU, memory, network usage
- **Business indicators**: order success rate, user retention, etc.

### 2. Rollback strategy

Roll back immediately when:
- The error rate exceeds a threshold (such as 1%)
- Significant increase in response time (e.g. >50%)
- System resources exhausted
- Abnormal business indicators

### 3. Pre-deployment check

- Code review completed
- Automated tests passed
- Performance benchmarks
- Security scan passed
- Document update completed

### 4. Post-deployment verification

- Check all monitoring indicators
- Validate key business processes
- Collect user feedback
- Update deployment documentation

## FAQ

Q: How long does it take to deploy a canary?
A: Depends on business traffic and verification needs, usually 2-4 hours.

Q: How is the duration of each phase determined?
A: Depending on business traffic volume and risk tolerance, high-traffic services require longer verification time.

Q: Can I skip certain stages?
A: Yes, but it will reduce security. It is recommended to go through at least three stages: small scale, medium scale and large scale.

Q: How to roll back after the canary version fails?
A: Set the canary weight in the xDS configuration to 0 to quickly roll back.

Q: Canary deployment of multiple services at the same time?
A: Not recommended. Canary deployment should be done on a service-by-service basis to avoid mutual impact.

Q: How to handle data migration?
A: During canary deployment, data version compatibility needs to be ensured, which usually requires the implementation of double-write or compatibility layers.
