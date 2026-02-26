# yggdrasil-ecosystem

Yggdrasil ecosystem integrations, examples, and supporting modules in a Go multi-module monorepo.

## Overview

The repository is organized by responsibility:

- `integrations/`: pluggable ecosystem integration modules for service platform capabilities.
- `examples/`: demo, validation, and compatibility assets for usage and verification.

## Integrations

| Module | Purpose | Docs |
| --- | --- | --- |
| `integrations/etcd` | etcd-based config source, service registry, and service discovery resolver | [integrations/etcd/README.md](./integrations/etcd/README.md) |
| `integrations/k8s` | Kubernetes service discovery resolver plus ConfigMap/Secret config sources | [integrations/k8s/README.md](./integrations/k8s/README.md) |
| `integrations/otlp` | OpenTelemetry OTLP exporters for traces and metrics | [integrations/otlp/README.md](./integrations/otlp/README.md) |
| `integrations/polaris` | Polaris registry, resolver, config source, and governance hooks | [integrations/polaris/README.md](./integrations/polaris/README.md) |
| `integrations/xds` | xDS resolver and balancer for dynamic traffic governance | [integrations/xds/README.md](./integrations/xds/README.md) |

## Quick Start

### For integration users

Install only the modules you need:

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/etcd/v2
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/k8s/v2
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/otlp/v2
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/polaris/v2
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/xds/v2
```

Then follow each module README for integration-specific configuration and runtime requirements.

### For contributors

```bash
make tools
make sync
make tidy
make fmt
make lint
make test
make coverage
```

To include example modules in test/coverage runs:

```bash
make test INCLUDE_EXAMPLES=1
make coverage INCLUDE_EXAMPLES=1
```

## Development

Requirements: Go 1.24+, Make, Python 3 (for pre-commit), and Docker for local integration scenarios.

Common scoped runs:

```bash
make MODULES="integrations/etcd" test
make MODULES="integrations/xds" lint
make MODULES="integrations/otlp integrations/otlp/example" test
```

## Documentation Index

- [integrations/etcd/README.md](./integrations/etcd/README.md): etcd integration details and configuration model.
- [integrations/k8s/README.md](./integrations/k8s/README.md): Kubernetes resolver and config-source usage.
- [integrations/otlp/README.md](./integrations/otlp/README.md): OTLP exporter configuration and behavior.
- [integrations/polaris/README.md](./integrations/polaris/README.md): Polaris integration and governance options.
- [integrations/xds/README.md](./integrations/xds/README.md): xDS resolver/balancer architecture and setup.

## Contributing

1. Run `make fmt`.
2. Run `make lint`.
3. Run `make test`.
4. Add or update tests when behavior changes.
5. Update docs when APIs/config/usage change.

Commit/branch conventions are validated by repository hooks and CI checks.

## License

Copyright 2022 The codesjoy Authors.

Licensed under the Apache License, Version 2.0.
