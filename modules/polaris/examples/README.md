# Polaris v3 Examples

This module intentionally keeps only v3-compilable examples. The previous v2
sample applications depended on v2-generated demo stubs and were removed during
the first Polaris v3 migration.

## Examples

- `quickstart`: end-to-end server registration and client discovery through a
  local Polaris standalone server.
- `multi-instance`: registers two instances of one service and demonstrates
  resolver refresh plus Polaris-backed client balancing.
- `metadata-discovery`: registers stable and canary instances, then filters
  discovery by Polaris metadata.
- `config-source`: seeds a Polaris Config Center file and loads it through
  `yggdrasil.config.sources` with `kind: polaris`.
- `governance`: configuration-only reference for Polaris routing, rate limiting,
  and circuit breaking.

All runnable examples reuse `quickstart/compose.yaml` to start the local Polaris
standalone server.
