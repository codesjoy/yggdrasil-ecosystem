# Polaris Example

Directory structure:

- `sample/server`: Start an RPC service and register it with Polaris (see sample/server/README.md)
- `sample/client`: Discover services and make RPC calls via Polaris (see sample/client/README.md)
- `scenarios/governance`: routing label example (client+server, see scenarios/governance/*/README.md)
- `scenarios/config-source`: Configuration center source example (see scenarios/config-source/README.md)
- `scenarios/instance-metadata`: instance metadata registration example (see scenarios/instance-metadata/server/README.md)
- `scenarios/multi-sdk`: Multi-SDKContext (distinguished by name) configuration instructions (see scenarios/multi-sdk/README.md)

Running mode (assuming Polaris Server is accessible, default `127.0.0.1:8091`):

- This example uses `yggdrasil.polaris.default.addresses` to initialize the SDKContext by default; you can also specify the Polaris native configuration file path in `yggdrasil.polaris.default.config_file` and configure the token through `yggdrasil.polaris.default.token`.
- If your Polaris configuration center address is different from the registered discovery address, configure `yggdrasil.polaris.default.config_addresses` (usually `host:8093`).

```bash
cd integrations/polaris/example/sample/server
go run ./
```

Open another terminal:

```bash
cd integrations/polaris/example/sample/client
go run ./
```

Governance routing label example:

```bash
cd integrations/polaris/example/scenarios/governance/server
go run ./
```

Open another terminal:

```bash
cd integrations/polaris/example/scenarios/governance/client
go run ./
```

Configuration center source example:

```bash
cd integrations/polaris/example/scenarios/config-source
go run ./
```
