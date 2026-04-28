# yggdrasil-ecosystem

Yggdrasil ecosystem modules in a Go multi-module monorepo.

This repository is centered on Yggdrasil v3 capability modules under `modules/`.
Each module owns its own `go.mod`, documentation, and runnable examples under
its local `examples/` directory. The root `Makefile` and `go.work` provide the
shared contributor workflow across modules.

## Overview

- `modules/*`: Yggdrasil v3 capability modules.
- `modules/*/examples`: runnable sample apps and configs for the owning module.
- `scripts/make-rules`: shared Make targets used by the root `Makefile`.
- `scripts/hooks`: commit and branch policy checks used by local hooks and CI.
- `go.work`: generated workspace file for local multi-module development.

## Modules

| Module | Purpose | Docs |
| --- | --- | --- |
| `modules/etcd` | etcd config source, registry, and resolver for Yggdrasil v3 | [modules/etcd/README.md](./modules/etcd/README.md) |
| `modules/k8s` | Kubernetes resolver plus ConfigMap/Secret config sources for Yggdrasil v3 | [modules/k8s/README.md](./modules/k8s/README.md) |
| `modules/otlp` | OpenTelemetry OTLP trace and metric providers for Yggdrasil v3 | [modules/otlp/README.md](./modules/otlp/README.md) |
| `modules/polaris` | Polaris registry, resolver, config source, and governance capabilities for Yggdrasil v3 | [modules/polaris/README.md](./modules/polaris/README.md) |
| `modules/protovalidate` | Buf Protovalidate server-side request validation interceptors for Yggdrasil v3 | [modules/protovalidate/README.md](./modules/protovalidate/README.md) |
| `modules/xds` | xDS resolver and balancer capabilities for Yggdrasil v3 dynamic traffic governance | [modules/xds/README.md](./modules/xds/README.md) |

Each module README is the source of truth for setup, configuration, and
module-specific examples.

## Quick Start

### For module users

Install only the module you need:

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3
```

Then follow that module's README for integration-specific configuration and
runtime requirements. Runnable examples live under the matching
`modules/*/examples` directory instead of a root-level `examples/` tree.

### For contributors

Use the standard root workflow:

```bash
make tools
make sync
make tidy
make fmt
make lint
make test
```

Run coverage when the change is behavior-heavy or when you need the coverage
gate locally:

```bash
make coverage
```

## Development

Requirements: Go 1.25+, Make, Python 3 for pre-commit, and Docker only when a
module's integration scenario requires a real external service.

Common scoped runs:

```bash
make MODULES="modules/k8s" test
make MODULES="modules/polaris" test
make MODULES="modules/otlp modules/otlp/examples" test
make MODULES="modules/xds" lint
```

To include example modules in repo-wide test or coverage runs:

```bash
make test INCLUDE_EXAMPLES=1
make coverage INCLUDE_EXAMPLES=1
```

Integration-tagged tests are opt-in:

```bash
make test TEST_TAGS=integration MODULES="modules/etcd"
make coverage TEST_TAGS=integration MODULES="modules/etcd"
```

## Testing Layout

- Unit tests live next to the code they exercise and use `*_test.go`.
- Package-level integration tests also live next to the code, use
  `*_integration_test.go`, and declare both `//go:build integration` and
  `// +build integration`.
- Use `<module>/test/integration/` only for module-level black-box integration
  tests that intentionally exercise public behavior across packages.
- Prefer embedded or in-process dependencies for package-level integration
  tests; use local Docker only when the scenario genuinely requires a real
  service, multi-process topology, or compatibility validation.
- Keep any Docker-backed test behind the `integration` build tag and document
  startup and cleanup flow in the owning module README or nearby test notes.

## Contributing

1. Run `make fmt`.
2. Run `make lint`.
3. Run `make test`.
4. Run `make test TEST_TAGS=integration MODULES="..."` when changing
   integration-tagged behavior.
5. Add or update tests when behavior changes.
6. Update docs when APIs, configuration, or examples change.

Commit and branch conventions are validated by repository hooks and CI checks.

## License

Copyright 2022 The codesjoy Authors.

Licensed under the Apache License, Version 2.0.
