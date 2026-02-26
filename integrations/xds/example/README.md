# xDS Examples

This directory contains complete examples of the integration of the yggdrasil v2 framework with the xDS protocol, demonstrating various service discovery, load balancing and traffic management scenarios.

## Directory structure

```
example/
├── README.md                    # This document
├── basic/                      # Basic integration example
├── canary/                     # Canary deployment example
├── load-balancing/             # Load balancing example
├── traffic-splitting/           # Traffic split example
├── multi-service/              # Multiple service example
└── control-plane/              # xDS control plane
```

## Sample overview

### 1. Basic - basic integration example

**Directory**: [basic/](./basic)

**Scenario**: Demonstrates the basic integration of xDS protocol, including service discovery, connection establishment and basic communication.

**Features**:
- xDS service discovery
- Dynamic configuration updates
- Complete RPC service implementation
- Metadata delivery
- Structured logs

**Run time**: ~10 seconds

**Suitable for the crowd**: Beginners, developers who are coming into contact with the xDS protocol for the first time

**Quick Start**:
```bash
cd control-plane
go run main.go --config config.yaml

cd basic/server
go run main.go --config config.yaml

cd basic/client
go run main.go --config config.yaml
```

**Test**: `./test.ps1`

---

### 2. Canary - Canary deployment example

**Directory**: [canary/](./canary)

**Scenario**: Demonstrates how to use xDS to implement progressive canary deployment to safely and gradually switch traffic from a stable version to a new version.

**Features**:
- Progressive traffic switching (5% → 100%)
- Multi-stage verification
- Real-time monitoring
- Safe rollback

**Running time**: ~1 minute (simulating 6 stages)

**Suitable for people**: DevOps engineers, teams that need a secure release process

**Quick Start**:
```bash
cd control-plane
set XDS_CONFIG_FILE=canary-xds-config.yaml
go run main.go --config config.yaml

cd canary/server
set DEPLOYMENT_TYPE=stable
go run main.go --config config.yaml

set DEPLOYMENT_TYPE=canary
set PORT=55556
go run main.go --config config.yaml

cd canary/client
go run main.go --config config.yaml
```

**Test**: `./test.ps1`

**Business Value**: Reduce release risk, quickly detect and fix issues

---

### 3. Load Balancing - Load balancing example

**Directory**: [load-balancing/](./load-balancing)

**Scenario**: Demonstrates different load balancing strategies including round robin, random and least requests.

**Features**:
- Multiple instance support
- Traffic statistics
- Multiple load balancing strategies
- Real-time monitoring

**Run time**: ~20 seconds (30 requests)

**Suitable for people**: Teams who need to optimize service performance and improve resource utilization

**Quick Start**:
```bash
cd control-plane
set XDS_CONFIG_FILE=load-balancing-xds-config.yaml
go run main.go --config config.yaml

cd load-balancing/server
set SERVER_ID=1
go run main.go --config config.yaml

set SERVER_ID=2
set PORT=55556
go run main.go --config config.yaml

set SERVER_ID=3
set PORT=55557
go run main.go --config config.yaml

cd load-balancing/client
go run main.go --config config.yaml
```

**Test**: `./test.ps1`

**Strategy comparison**:
- **Round Robin**: fair distribution, suitable for instances with similar performance
- **Random**: simple and efficient, suitable for scenarios with a large number of instances
- **Least Request**: Dynamic adaptation, suitable for scenarios with large differences in request processing time

---

### 4. Traffic Splitting - Traffic splitting example

**Directory**: [traffic-splitting/](./traffic-splitting)

**Scenario**: Demonstrates how to allocate traffic to different backend services based on weight.

**Features**:
- Traffic segmentation (distribution by weight)
- Real-time statistics
- Flexible configuration
- Multiple backend support

**Run time**: ~15 seconds (20 requests)

**Suitable for people**: Teams who need A/B testing, canary release, and capacity management

**Quick Start**:
```bash
cd control-plane
set XDS_CONFIG_FILE=traffic-splitting-xds-config.yaml
go run main.go --config config.yaml

cd traffic-splitting/server
set BACKEND_ID=1
go run main.go --config config.yaml

set BACKEND_ID=2
set PORT=55556
go run main.go --config config.yaml

cd traffic-splitting/client
go run main.go --config config.yaml
```

**Test**: `./test.ps1`

**Application Scenario**:
- A/B testing: 50% / 50%
- Grayscale publishing: 90% / 10% → 0% / 100%
- Capacity management: allocate traffic based on backend capacity
- Multi-tenancy: different tenants use different backends

---

### 5. Multi-Service - Multi-service example

**Directory**: [multi-service/](./multi-service)

**Scenario**: Demonstrates how to implement service discovery and communication in a multi-service scenario.

**Features**:
- Multi-service support
- Unified management
- Service discovery
- Metadata delivery
- Flexible configuration

**Running time**: ~10 seconds (20 requests, calling 2 services alternately)

**Suitable for people**: Microservice architecture developers, teams that need to integrate multiple services

**Quick Start**:
```bash
cd control-plane
set XDS_CONFIG_FILE=multi-service-xds-config.yaml
go run main.go --config config.yaml

cd multi-service/server
go run main.go --config config.yaml

cd multi-service/client
go run main.go --config config.yaml
```

**Test**: `./test.ps1`

**Application Scenario**:
- Microservice architecture: multiple independent services working together
- Gateway mode: unified service management and configuration
- Multi-tenant system: different tenants use different service instances
- Hybrid deployment: local service + remote service

---

## Control Plane - xDS control plane

**Directory**: [control-plane/](./control-plane)

**Function**: Provides xDS configuration management and service discovery functions.

**characteristic**:
- Modular architecture (server/, snapshot/, watcher/)
- SnapshotCache implementation (officially recommended by Envoy)
- File monitoring and hot reloading (fsnotify)
- YAML configuration driver
- Graceful shutdown and signal handling
- Callback system and monitoring

**Configuration file**:
- `config.yaml`: Control plane server configuration
- `xds-config.yaml`: Default xDS configuration
- `canary-xds-config.yaml`: Canary deployment configuration
- `load-balancing-xds-config.yaml`: Load balancing configuration
- `traffic-splitting-xds-config.yaml`: Traffic splitting configuration
- `multi-service-xds-config.yaml`: Multi-service configuration

**Port**: 18000

**Startup method**:
```bash
cd control-plane
# Use default configuration
go run main.go --config config.yaml

# Use specific configuration
set XDS_CONFIG_FILE=canary-xds-config.yaml
go run main.go --config config.yaml
```

---

## Quick Start Guide

### 1. Environment preparation

Make sure it is installed:
- Go 1.21+
- PowerShell 5.1+

### 2. Select an example

Choose the appropriate example based on your needs:

| Requirements | Recommended examples |
|------|----------|
| Learn the basics of xDS | Basic |
| Release new versions safely | Canary |
| Optimize service performance | Load Balancing |
| A/B testing or canary publishing | Traffic Splitting |
| Integrate multiple services | Multi-Service |

### 3. Run the example

Each example includes a detailed README document, just follow the document steps to run it.

### 4. Run the test

Automated test scripts are provided for each example:

```powershell
cd <example-directory>
.\test.ps1
```

---

## FAQ

### Q: Does the control plane have to run on port 18000?

A: No. The `server.port` configuration can be modified in `control-plane/config.yaml`.

### Q: Can I run multiple examples at the same time?

A: Not recommended. Each example uses a different xDS configuration, running them simultaneously may cause conflicts. It is recommended to run the examples one at a time.

### Q: Can the service endpoint address in the example be modified?

A: Yes. Modify the address and port of endpoints in the corresponding xDS configuration file (such as `canary-xds-config.yaml`).

### Q: How to debug xDS communication issues?

A: Enable detailed logging:

```yaml
yggdrasil:
  logger:
    handler:
      default:
        level: "debug"
```

### Q: What should I do if the test script fails?

A: Check the following points:
1. Is the control plane running?
2. Is the server running?
3. Whether the port is occupied
4. View the log file (*.log) output by the test script

### Q: Can these examples be used in a production environment?

A: These examples are mainly for learning and demonstration. Before using it in a production environment, you need to:
1. Add a complete error handling and retry mechanism
2. Implement health check and failover
3. Add monitoring and alarms
4. Implement configuration version management
5. Add security configuration (TLS, authentication, etc.)

---

## Technology stack

- **Framework**: yggdrasil v2
- **Protocol**: gRPC, xDS v3
- **Configuration**: YAML
- **Log**: slog (Go 1.21+)
- **File monitoring**: fsnotify
- **xDS library**: envoy go-control-plane v3

---

## Further reading

- [yggdrasil documentation](https://github.com/codesjoy/yggdrasil)
- [Envoy xDS Protocol Document](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [gRPC Documentation](https://grpc.io/docs/)
- [Go control plane library](https://github.com/envoyproxy/go-control-plane)

---

## contribute

You are welcome to submit Issues and Pull Requests to improve these examples.

---

## License

Aligned with the yggdrasil project.
