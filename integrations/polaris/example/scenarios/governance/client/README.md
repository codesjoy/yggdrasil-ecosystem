# Governance Client (request label routing example)

This example demonstrates: transparently transmitting the "request tag" (such as `env/lane/user_id`) through out-metadata, and using `polaris` balancer/picker to perform route filtering and point selection.

The key point is: configure `skipRouteFilter: true` on the resolver side to avoid static route filtering first, and leave the decision of "per request label" to the picker.

## Preconditions

1. Started [server](../server/README.md) and confirmed that at least 1 instance exists in Polaris.
2. Accessible Polaris Server (default `8091`), and console (default `8080`, used to configure routing rules).

## Startup method

1. Modify [config.yaml](config.yaml):
- `yggdrasil.polaris.default.addresses`: Polaris naming address
- `yggdrasil.polaris.default.token`: optional
- `yggdrasil.client.<serverName>.balancer: polaris`: enable polaris picker
2. Start:

```bash
cd integrations/polaris/example/scenarios/governance/client
go run ./
```

## Where does the request tag come from?

In [main.go](main.go#L38-L43), the client will bring the following out-metadata to the request context:

- `env=dev`
- `lane=stable`
- `user_id=123`

These tags will be used as input to the "request tag route" by the `polaris` picker.

## Polaris Console: Create a Minimal Routing Rule

Goal: Let "request with `lane=stable`" only hit instances of `lane=stable`.

Recommended preparation:
- Start at least two servers (see "Preparing two sets of instances for canary" in the server README) and register `lane=stable` and `lane=canary` respectively.

Operation steps (the console menu names may be slightly different in different versions):

1. Open the console: `http://127.0.0.1:8080`
2. Enter "Service Management/Service List" and find the target service:
   - `github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server`
3. Enter "Governance/Routing Rules" (or "Traffic Management/Routing")
4. Create rules (illustration):
- Rule condition: Request tag `lane = stable`
- Route target: instance metadata `lane = stable`
5. Save and publish the rule
6. Rerun the client and observe:
- If you print the local port/instance information on the server side (not printed in this example), you can confirm the hit instance through the console/monitoring

## Configuration instructions (key points)

### 1) Resolver

- `protocols: ["grpc"]`: Only deliver gRPC instances
- `skipRouteFilter: true`: Leave request label routing to picker

### 2) Governance switch (the example only turns on routing by default)

`yggdrasil.polaris.governance.config.routing.enable: true`

- `rateLimit/circuitBreaker` is turned off by default; if you want to demonstrate rate limiting/fuse, you need to configure the corresponding rules in the Polaris console first, and then turn on the switch.

## FAQ

- Routing rules take effect but still hit "non-stable" instances:
- Confirm that there is indeed `lane` in the instance metadata (set in the server's `yggdrasil.application.metadata`).
- Make sure the resolver is configured as `skipRouteFilter: true` (this is the default in the example).
- Confirm routing rules are "published" instead of just saving a draft.
- Confirm that the key/value used by the console rule is exactly the same as the request out-metadata (case sensitive).
- If the rule does not hit any "ready" instance, the client will fall back to the normal selection point; at this time, it is necessary to check whether the instance is healthy and whether the connection is Ready.
