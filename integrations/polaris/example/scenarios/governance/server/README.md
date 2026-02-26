# Governance Server (registered with instance metadata)

This example starts a gRPC service and registers it with Polaris. It also demonstrates how to inject namespace/version/metadata through `yggdrasil.application`, which is ultimately reflected in the instance metadata of Polaris for use by routing/canary and other rules.

## What you get

- Service: `github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server`
- Instance metadata (example):
  - `env=dev`
  - `lane=stable`
  - `version=v1.0.0`

## Startup method

1. Modify [config.yaml](config.yaml):
- `yggdrasil.polaris.default.addresses`: Polaris naming address (usually `host:8091`)
- `yggdrasil.polaris.default.token`: optional
- `yggdrasil.registry.config.namespace`: the namespace registered to (example `default`)
- `yggdrasil.remote.protocol.grpc.address`: local listening address (example `127.0.0.1:55880`)
2. Start:

```bash
cd integrations/polaris/example/scenarios/governance/server
go run ./
```

## Configuration instructions (key points)

### 1) Instance information injection (yggdrasil.application)

```yaml
yggdrasil:
  application:
    namespace: "default"
    version: "v1.0.0"
    metadata:
      env: "dev"
      lane: "stable"
```

- `namespace`: Affects the default registry namespace (if registry config does not specify it explicitly).
- `version` / `metadata`: Will be merged into the registration request as instance metadata for use by Polaris routing/canary/grouping and other capabilities.

### 2) Registry registration

```yaml
yggdrasil:
  registry:
    type: polaris
    config:
      namespace: "default"
```

## Polaris Console Verification

1. Open the console: `http://127.0.0.1:8080`
2. Enter "Service Management/Service List" and find:
   - `github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server`
3. Open the "Instances" page of the service details:
- Confirm that the instance port is `55880`
- Confirm that the instance metadata contains `env/lane` (and version)

## Optional: Prepare two sets of instances for canary

If you want to visually see the effect of "routing by label" in routing rules, it is recommended to start two servers:

1. Copy a directory or temporarily change the config:
- Copy 1: `lane=stable`, port `55880`
- Copy 2: `lane=canary`, port `55882`
2. Both copies are registered under the same service name and namespace.
3. Configure routing rules in the Polaris console (see client README)

Notice:
- Both instances need to be in a healthy state; if all instances of a certain tag group are unavailable and the route cannot be hit, the normal selection point will be returned.
- Just changing `lane` is not enough. The second instance needs to change `yggdrasil.remote.protocol.grpc.address` at the same time to avoid port conflicts.
