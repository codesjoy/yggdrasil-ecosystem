# Instance Metadata (namespace/version/metadata registration example)

This example demonstrates setting the application dimension's `namespace/version/metadata` via `yggdrasil.application` and bringing this information to the instance when registering with Polaris.

## What you get

- Service: `github.com.codesjoy.yggdrasil.contrib.polaris.example.instance_metadata.server`
- Register to namespace: `prod` (example configuration)
- Instance metadata (example):
  - `env=prod`
  - `region=gz`
  - `lane=blue`
  - `version=v2026.01`

## Startup method

1. Modify [config.yaml](config.yaml):
- `yggdrasil.polaris.default.addresses`: Polaris naming address (usually `host:8091`)
- `yggdrasil.application.namespace/version/metadata`: Adjust according to your environment
- `yggdrasil.registry.config.namespace`: It is recommended to be consistent with application.namespace (example is `prod`)
- `yggdrasil.remote.protocol.grpc.address`: local listening address (example `127.0.0.1:55881`)
2. Start:

```bash
cd integrations/polaris/example/scenarios/instance-metadata/server
go run ./
```

## Configuration instructions (key points)

### 1) yggdrasil.application

```yaml
yggdrasil:
  application:
    namespace: "prod"
    version: "v2026.01"
    metadata:
      env: "prod"
      region: "gz"
      lane: "blue"
```

- `namespace`: As the application default namespace; it will fall back to use when the registry does not explicitly specify the namespace.
- `version`/`metadata`: Will participate in the registration of instance metadata, used for routing/canary/grouping, and console display retrieval.

### 2) registry namespace

In the example, registration to `prod` is explicitly specified:

```yaml
yggdrasil:
  registry:
    config:
      namespace: "prod"
```

## Polaris Console Verification

1. Open the console: `http://127.0.0.1:8080`
2. Switch the namespace to `prod` (if the console supports multiple namespaces)
3. Search for services in "Service Management/Service List":
   - `github.com.codesjoy.yggdrasil.contrib.polaris.example.instance_metadata.server`
4. Open the "Instances" page of the service details:
- Confirm that the instance port is `55881`
- Check whether the instance metadata contains `env/region/lane/version`

## FAQ

- The console cannot see the service:
- Make sure you switch to the correct namespace (example is `prod`).
- Verify that `registry.config.namespace` does not conflict with `application.namespace`.
