# Sample Client (discovered and called via Polaris)

The example starts a gRPC client, gets a list of instances through the Polaris resolver, performs route/load balancing selection using the `polaris` balancer (picker), and then makes an RPC call.

## What you get

- Discover services via Polaris: `github.com.codesjoy.yggdrasil.contrib.polaris.example.server`
- Initiate two RPCs:
- `GetShelf`: Success, and print header/trailer
- `MoveBook`: Return business error with reason and print reason/code/httpCode

## Preconditions

1. First start [sample/server](../server/README.md) and confirm it is registered to Polaris.
2. An accessible Polaris Server (the example uses `8091` by default).

## Startup method

1. Modify [config.yaml](config.yaml) of this directory:
- Change `yggdrasil.polaris.default.addresses` to your Polaris address (e.g. `127.0.0.1:8091`).
- If authentication is required, fill in `yggdrasil.polaris.default.token`.
2. Start client:

```bash
cd integrations/polaris/example/sample/client
go run ./
```

## Configuration instructions (core fields)

### 1) Polaris SDK (connect to Polaris)

- `yggdrasil.polaris.default.addresses`: Register discovery address (naming).
- `yggdrasil.polaris.default.token`: Optional.

### 2) Resolver (pulling instances from Polaris)

- `yggdrasil.resolver.default.type: polaris`
- `yggdrasil.resolver.default.config.namespace`: The namespace where the target service is located (example is `default`).
- `yggdrasil.resolver.default.config.protocols: ["grpc"]`: Only deliver gRPC instances to avoid mixing non-RPC protocol instances.
- `refreshInterval`: The period for regularly pulling the instance list.
- `skipRouteFilter: true`：
- It is recommended to keep it true and let the "by request label" route filtering be done by picker (requires out-metadata).

### 3) Client binds resolver + balancer

```yaml
yggdrasil:
  client:
    github.com.codesjoy.yggdrasil.contrib.polaris.example.server:
      resolver: default
      balancer: polaris
```

- `resolver`: Which resolver instance name to choose (here `default`).
- `balancer: polaris`: Enable polaris picker (support routing/rate limiting/fuse closed loop).

### 4) Management switch (optional)

In the example, only routing is enabled by default:

- `yggdrasil.polaris.governance.config.routing.enable: true`
- `rateLimit/circuitBreaker` is turned off by default (you need to configure the corresponding rules in the Polaris console before turning it on)

## Polaris console operations (minimum steps)

1. Enter "Service Management/Service List" and confirm that the service exists and there is at least 1 healthy instance.
2. (Optional) If you want to verify routing capabilities:
- Enter "Governance/Routing Rules" (the console menu name may be slightly different in different versions)
- Create routing rules for the target service: filter by request tag (such as `env=dev`) or instance metadata
- Rerun the client and observe whether the corresponding instance is selected according to the rules.

## Key points of the code (convenient for you to modify)

- The request tag is transparently transmitted to the picker through out-metadata:
- see [main.go](main.go#L38-L42)
- header/trailer reads from ctx:
- see [main.go](main.go#L46-L52)
