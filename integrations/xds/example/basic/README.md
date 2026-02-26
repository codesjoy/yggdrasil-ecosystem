#Basic integration example

This example demonstrates how to use the yggdrasil v2 framework to implement basic integration of the xDS protocol, including service discovery, connection establishment and basic communication.

## Directory structure

```
basic/
├── client/
│   ├── main.go      # Client implementation
│   └── config.yaml  # Client configuration
└── server/
    ├── main.go      # Server side implementation
    └── config.yaml  # Server configuration
```

## Features

- **xDS Service Discovery**: Discover service endpoints through the xDS control plane
- **Dynamic Configuration**: Supports dynamically updating service configuration at runtime
- **Complete service implementation**: Implements all RPC methods of Library service
- **Metadata transfer**: Use metadata to transfer service identification information
- **Structured Logging**: Complete request/response logging

## Run steps

### 1. Start the xDS control plane

```bash
cd integrations/xds/example/control-plane
go run main.go --config config.yaml
```

The control plane will be started on port 18000.

### 2. Start the server

```bash
cd integrations/xds/example/basic/server
go run main.go --config config.yaml
```

The server will start on port 55555.

### 3. Run the client

```bash
cd integrations/xds/example/basic/client
go run main.go --config config.yaml
```

## Expected output

### Server log

```
time=2025-01-26T10:00:00.000Z level=INFO msg="GetShelf called" name=shelves/1
time=2025-01-26T10:00:00.100Z level=INFO msg="CreateShelf called" name=shelves/2 theme=History
time=2025-01-26T10:00:00.200Z level=INFO msg="ListShelves called" parent=""
```

### Client log

```
time=2025-01-26T10:00:00.000Z level=INFO msg="Starting xDS basic client..."
time=2025-01-26T10:00:00.050Z level=INFO msg="Calling GetShelf..."
time=2025-01-26T10:00:00.150Z level=INFO msg="GetShelf response" name=shelves/1 theme="Basic Service Theme"
time=2025-01-26T10:00:00.150Z level=INFO msg="Response trailer" trailer="map[server:basic-server]"
time=2025-01-26T10:00:00.150Z level=INFO msg="Response header" header="map[server:basic-server]"
time=2025-01-26T10:00:00.200Z level=INFO msg="Calling CreateShelf..."
time=2025-01-26T10:00:00.300Z level=INFO msg="CreateShelf response" name=shelves/2 theme=History
time=2025-01-26T10:00:00.350Z level=INFO msg="Calling ListShelves..."
time=2025-01-26T10:00:00.450Z level=INFO msg="ListShelves response" count=2
time=2025-01-26T10:00:00.450Z level=INFO msg="Shelf" index=0 name=shelves/1 theme="Basic Service Theme 1"
time=2025-01-26T10:00:00.450Z level=INFO msg="Shelf" index=1 name=shelves/2 theme="Basic Service Theme 2"
time=2025-01-26T10:00:00.500Z level=INFO msg="xDS basic client completed successfully"
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

  client:
    github.com.codesjoy.yggdrasil.example.sample:
      resolver: "xds"
      balancer: "xds"

  xds:
    default:
      server:
        address: "127.0.0.1:18000"  # xDS control plane address
      node:
        id: "basic-client"              # Client node ID
        cluster: "test-cluster"          # Cluster name
      protocol: "grpc"
```

### Server configuration (config.yaml)

```yaml
yggdrasil:
  server:
    protocol:
      - "grpc"
  remote:
    protocol:
      grpc:
        address: "127.0.0.1:55555"  # gRPC service port
```

## Technical points

### 1. yggdrasil initialization

```go
if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
    os.Exit(1)
}

if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.basic.client"); err != nil {
    os.Exit(1)
}
```

### 2. Create client

```go
cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
if err != nil {
    os.Exit(1)
}
defer cli.Close()
```

### 3. Use metadata

```go
ctx := metadata.WithStreamContext(context.Background())
client := librarypb.NewLibraryServiceClient(cli)
```

### 4. Start the server

```go
ss := &LibraryImpl{}
if err := yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
); err != nil {
    os.Exit(1)
}
```

## Supported RPC methods

| Method | Description |
|------|------|
| CreateShelf | Create bookshelf |
| GetShelf | Get bookshelf information |
| ListShelves | List all bookshelves |
| DeleteShelf | Delete bookshelf |
| MergeShelves | Merge Bookshelf |
| CreateBook | Create book |
| GetBook | Get book information |
| ListBooks | List books in bookshelf |
| DeleteBook | Delete book |
| UpdateBook | Update book information |
| MoveBook | Move books to other bookshelf |

## Debugging Tips

### Enable detailed logging

Modify the log level in the configuration file:

```yaml
yggdrasil:
  logger:
    handler:
      default:
        level: "debug"
```

### View xDS communication

View the request in the control plane log:

```bash
[xDS Control Plane] Request from node: basic-client, type: type.googleapis.com/envoy.config.listener.v3.Listener
```

### Verify service discovery

Check that the client is properly connected to the service:

```bash
grpcurl -plaintext 127.0.0.1:55555 list
```

## FAQ

Q: Client cannot connect to xDS control plane?
A: Check whether the control plane is started on port 18000 and whether the address configured on the client is correct.

Q: The server did not receive the request?
A: Confirm that the xDS control plane has correctly configured the service endpoint information and check whether the server is started on port 55555.

Q: How to switch to other xDS control plane?
A: Modify `xds.default.server.address` in the client configuration.

Q: How to add a new service method?
A: Define the method in the proto file, regenerate the code, and then implement the new method in `LibraryImpl`.
