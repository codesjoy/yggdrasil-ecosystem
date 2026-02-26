#Multi-service example

This example demonstrates how to use the yggdrasil v2 framework and xDS to implement service discovery and communication in a multi-service scenario.

## Directory structure

```
multi-service/
├── client/
│   ├── main.go      # Client implementation, calling multiple services
│   └── config.yaml  # Client configuration
└── server/
    ├── main.go      # Server-side implementation, providing multiple services at the same time
    └── config.yaml  # Server configuration
```

## Features

- **Multi-service support**: The client can call multiple different gRPC services at the same time
- **Unified Management**: Use yggdrasil to centrally manage the connections and configuration of all services
- **Service Discovery**: Discover the endpoint information of all services through xDS
- **Metadata Passing**: Use metadata to pass stream identifiers
- **Flexible Configuration**: Supports dynamic configuration of xDS settings for multiple services

## Run steps

### 1. Start the xDS control plane

```bash
cd integrations/xds/example/control-plane
set XDS_CONFIG_FILE=multi-service-xds-config.yaml
go run main.go --config config.yaml
```

### 2. Start the multi-service server

```bash
cd integrations/xds/example/multi-service/server
go run main.go --config config.yaml
```

### 3. Run the client

```bash
cd integrations/xds/example/multi-service/client
go run main.go --config config.yaml
```

## Expected output

The client will alternately call the Library service and the Greeter service:

```
2025/01/26 10:00:00 INFO Starting multi-service client...
2025/01/26 10:00:00 INFO Starting multi-service test loop...
2025/01/26 10:00:00 INFO Greeter service response message="Hello, world"
2025/01/26 10:00:00 INFO Library service response name="shelf-" theme=""
2025/01/26 10:00:00 INFO Greeter service response message="Hello, world"
2025/01/26 10:00:00 INFO Library service response name="shelf-" theme=""
...
2025/01/26 10:00:09 INFO Multi-service client completed successfully
```

## Technical architecture

### Service architecture

```
┌─────────────────┐
│   Client        │
│                 │
│ ┌─────────────┐ │
│ │ Library     │ │
│ │ Service     │ │
│ │ Client      │ │
│ └─────────────┘ │
│                 │
│ ┌─────────────┐ │
│ │ Greeter     │ │
│ │ Service     │ │
│ │ Client      │ │
│ └─────────────┘ │
└────────┬────────┘
         │
         │ xDS Discovery
         │
┌────────▼────────┐
│  xDS Control     │
│  Plane           │
└────────┬────────┘
         │
         │ Service Discovery
         │
┌────────▼────────┐
│  Multi-Service   │
│  Server          │
│                  │
│  - Library Svc   │
│  - Greeter Svc   │
└──────────────────┘
```

### Configuration structure

```yaml
yggdrasil:
  client:
    # Library service configuration
    github.com.codesjoy.yggdrasil.example.library:
      resolver: "xds"
      balancer: "xds"

    # Greeter service configuration
    github.com.codesjoy.yggdrasil.example.greeter:
      resolver: "xds"
      balancer: "xds"

  xds:
    default:
      server:
        address: "127.0.0.1:18000"
      node:
        id: "multi-service-client"
        cluster: "test-cluster"
      protocol: "grpc"
```

## Configuration instructions

### Client configuration (config.yaml)

```yaml
yggdrasil:
  resolver:
    xds:
      type: "xds"
      config:
        name: "default"

  # Configuring clients for multiple services
  client:
    github.com.codesjoy.yggdrasil.example.library:
      resolver: "xds"
      balancer: "xds"
    github.com.codesjoy.yggdrasil.example.greeter:
      resolver: "xds"
      balancer: "xds"

  xds:
    default:
      server:
        address: "127.0.0.1:18000"
      node:
        id: "multi-service-client"
        cluster: "test-cluster"
      protocol: "grpc"
```

### xDS configuration (multi-service-xds-config.yaml)

```yaml
clusters:
  # Library service cluster
  - name: "library-cluster"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"

  # Greeter service cluster
  - name: "greeter-cluster"
    connectTimeout: "5s"
    type: "EDS"
    lbPolicy: "ROUND_ROBIN"

endpoints:
  # Library service endpoint
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55890

  # Greeter service endpoint
  - clusterName: "greeter-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55890

listeners:
  # Library service listener
  - name: "library-service"
    address: "0.0.0.0"
    port: 10000
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "library-route"

  # Greeter service listener
  - name: "greeter-service"
    address: "0.0.0.0"
    port: 10001
    filterChains:
      - filters:
          - name: "envoy.filters.network.http_connection_manager"
            routeConfigName: "greeter-route"

routes:
  # Library service routing
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

  # Greeter service routing
  - name: "greeter-route"
    virtualHosts:
      - name: "greeter"
        domains: ["*"]
        routes:
          - match:
              path:
                prefix: "/"
            route:
              cluster: "greeter-cluster"
```

## Technical points

### 1. Create multiple clients

```go
libraryClient, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.library")
if err != nil {
    os.Exit(1)
}
defer libraryClient.Close()

greeterClient, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.greeter")
if err != nil {
    os.Exit(1)
}
defer greeterClient.Close()
```

### 2. Create multiple service clients

```go
library := libraryv1.NewLibraryServiceClient(libraryClient)
greeter := helloworldv1.NewGreeterServiceClient(greeterClient)
```

### 3. Call services alternately

```go
for i := 1; i <= 20; i++ {
    time.Sleep(500 * time.Millisecond)

    if i%2 == 0 {
        ctx := metadata.WithStreamContext(context.Background())
        shelf, err := library.GetShelf(ctx, &libraryv1.GetShelfRequest{
            Name: "shelf-" + string(rune(i)),
        })
        slog.Info("Library service response", "name", shelf.Name, "theme", shelf.Theme)
    } else {
        ctx := metadata.WithStreamContext(context.Background())
        response, err := greeter.SayHello(ctx, &helloworldv1.SayHelloRequest{
            Name: "world",
        })
        slog.Info("Greeter service response", "message", response.Message)
    }
}
```

### 4. Multi-service implementation

The server registers two services at the same time:

```go
libraryImpl := &librarypb2.LibraryServiceServer{}
greeterImpl := &greeterpb2.GreeterServiceServer{}

if err := yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, libraryImpl),
    yggdrasil.WithServiceDesc(&greeterpb2.GreeterServiceServiceDesc, greeterImpl),
); err != nil {
    os.Exit(1)
}
```

## Inter-service communication

### Synchronous call

```go
ctx := metadata.WithStreamContext(context.Background())
response, err := greeter.SayHello(ctx, &helloworldv1.SayHelloRequest{
    Name: "world",
})
```

### Error handling

```go
response, err := greeter.SayHello(ctx, req)
if err != nil {
    slog.Error("Greeter service call failed", "error", err)
    continue
}
```

### Resource Cleanup

```go
defer libraryClient.Close()
defer greeterClient.Close()
```

## Application scenarios

### 1. Microservice architecture

- Multiple independent services working together
- Communication between services through gRPC
- Unified service discovery and load balancing

### 2. Gateway mode

- The client calls multiple backend services
- Unified service management and configuration
- Simplified client logic

### 3. Multi-tenant system

- Different tenants use different service instances
- Dynamically configure tenant routing through xDS
- Flexible service isolation and sharing

### 4. Hybrid deployment

- Some services are deployed locally
- Remote calling of some services
- Unified service management framework

## Best Practices

### 1. Service naming

Use clear and consistent service naming conventions:
- `github.com.codesjoy.yggdrasil.example.library`
- `github.com.codesjoy.yggdrasil.example.greeter`

### 2. Configuration management

- All services use unified xDS configuration
- Service specific configuration is set in the client configuration
- Avoid duplicate configuration

### 3. Error handling

- Errors must be handled individually for each service call
- Implement retry and circuit breaker mechanisms
- Record detailed error logs

### 4. Resource Management

- Make sure all clients are closed properly
- Use defer to ensure resource release
- Avoid connection leaks

### 5. Monitoring and Tracking

- Add monitoring for each service call
- Use distributed tracing to track calls between services
- Collect service performance metrics

## FAQ

Q: Can one client connect to multiple services?
A: Yes. yggdrasil supports the creation of multiple client instances, each instance connecting to a different service.

Q: Do multiple services have to be deployed on the same server?
A: No need. Services can be deployed on different servers and endpoint information is configured through xDS.

Q: How to manage multiple service versions?
A: Use the version control function of xDS to configure different versions for each service.

Q: How to set the timeout for calls between services?
A: Set the timeout in the context, or set it uniformly through the configuration of yggdrasil.

Q: How to achieve load balancing between services?
A: Configure the load balancing policy for each service in the xDS configuration.

Q: How is the performance in a multi-service scenario?
A: The performance is good. yggdrasil uses connection pooling and concurrency control to efficiently handle concurrent calls to multiple services.

Q: How to debug multiple service calls?
A: Use structured logs to record each service call, including requests, responses, and error messages. Use distributed tracing tools to track cross-service calls.
