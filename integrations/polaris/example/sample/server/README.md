# Sample Server (registered to Polaris)

This example starts a gRPC service and registers the instance with Polaris via Yggdrasil's `registry=polaris`.

## What you get

- A service: `github.com.codesjoy.yggdrasil.contrib.polaris.example.server`
- An instance: visible in Polaris "Service List/Instance List" (host/port/protocol)

## Preconditions

1. An accessible Polaris Server (the example defaults to gRPC registration discovery port `8091`).
2. The namespace has been prepared (`default` is used in the example).
3. Optional: If your Polaris has authentication turned on, you need to prepare token (corresponding to `yggdrasil.polaris.default.token`) and service token (corresponding to `yggdrasil.registry.config.serviceToken`).

### Quickly start Polaris (optional)

Development testing can be started using the official stand-alone image (which will also include console/server/limiter/prometheus and other components):

```bash
docker run -d --privileged=true --name polaris-standalone \
  -p 8080:8080 \
  -p 8090:8090 \
  -p 8091:8091 \
  -p 8093:8093 \
  polarismesh/polaris-standalone:latest
```

- Console: `http://127.0.0.1:8080`
- Register discovery gRPC: `127.0.0.1:8091`
- Configuration Center gRPC: `127.0.0.1:8093`

## Startup method

1. Modify [config.yaml](config.yaml) of this directory:
- Change `yggdrasil.polaris.default.addresses` to your Polaris Server address (e.g. `127.0.0.1:8091`).
- If authentication is not required: delete or leave `yggdrasil.polaris.default.token` blank, and leave `yggdrasil.registry.config.serviceToken` blank.
2. Start the service:

```bash
cd integrations/polaris/example/sample/server
go run ./
```

After successful startup, the service will listen at `yggdrasil.remote.protocol.grpc.address` (default `:55879`).

## Configuration instructions (core fields)

### 1) Polaris SDK (connect to Polaris)

- `yggdrasil.polaris.default.addresses`: Polaris registration discovery address list (usually `host:8091`).
- `yggdrasil.polaris.default.token`: Polaris access token (optional).

### 2) Registry (register the instance to Polaris)

- `yggdrasil.registry.type: polaris`
- `yggdrasil.registry.config.sdk: default`: Which set of SDK configuration to use (corresponding to `yggdrasil.polaris.<name>`).
- `yggdrasil.registry.config.namespace`: Which namespace to register to (if not filled in, it will fall back to the application namespace, and then fall back to `default`).
- `yggdrasil.registry.config.serviceToken`: Service-level token (optional, depending on whether Polaris enables authentication).
- `ttl/autoHeartbeat`: Whether to enable heartbeat and instance TTL.

### 3) Application listening address

- `yggdrasil.remote.protocol.grpc.address`: The local gRPC listening address. The host/port of instances registered to Polaris will be derived based on this.

## Polaris console operations (recommended process)

1. Open the console: `http://127.0.0.1:8080`
2. Select/Create a namespace:
- Confirm that `default` (or the namespace you filled in the configuration) exists.
3. Check whether the service appears:
- Enter "Service Management/Service List" and search for the service name `github.com.codesjoy.yggdrasil.contrib.polaris.example.server`.
4. Check whether the instance is online:
- Open the "Instances" page of the service details and confirm that there is 1 instance record, the port is `55879`, and the protocol is `grpc`.
5. (Optional) If you need a service token:
- Find the token configuration/display position in the service details, fill in the token back to `serviceToken` and restart the service.

## FAQ

- The service is registered but cannot be found by the client:
- Make sure server/client uses the same namespace.
- Confirm that the instance protocol registered by the server is `grpc` (the example will fill in the endpoint scheme as `grpc`).
- Authentication failed:
- Confirm the sources and permissions of `yggdrasil.polaris.default.token` (platform token) and `serviceToken` (service token) respectively.
