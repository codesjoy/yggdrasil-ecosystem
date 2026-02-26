# Config Source (loaded from Polaris Configuration Center and subscribed)

This example demonstrates: loading a configuration file in the Polaris configuration center as Yggdrasil's `config source`, subscribing to the change event, and driving the `config.AddWatcher` callback output.

## What will you see

- Will block running after startup (`select {}`).
- After you modify and publish the configuration file in the Polaris console, the terminal will output something like:

```text
type: update version: <n> value: <int>
```

Among them, `value` comes from the monitored key in your configuration file (the default key is `dd`).

## Preconditions

1. Accessible Polaris Server (the default gRPC port of the configuration center is `8093`, and the default gRPC port of registration discovery is `8091`).
2. The namespace has been prepared (`default` is used in the example).
3. (Optional) If authentication is enabled, prepare token (`yggdrasil.polaris.default.token`).

## Startup method

1. Modify this directory [config.yaml](config.yaml):
- `yggdrasil.polaris.default.addresses`: naming address (usually `host:8091`)
- `yggdrasil.polaris.default.config_addresses`: config address (usually `host:8093`)
- `yggdrasil.polaris.default.token`: optional
2. Start:

```bash
cd integrations/polaris/example/scenarios/config-source
go run ./
```

## Polaris console operations (create and publish profiles)

The goal is to create a configuration file: `namespace=default`, `fileGroup=app`, `fileName=service.yaml`, which contains the monitored key (default `dd`).

1. Open the console: `http://127.0.0.1:8080`
2. Enter "Configuration Center/Configuration File" (the console menu name may be slightly different in different versions)
3. Select namespace `default`
4. Create a configuration file:
- Group (FileGroup): `app`
- FileName: `service.yaml`
5. Edit content (example) and publish:

```yaml
dd: 1
```

6. Return to the sample process output window: the initial value (or first changed value) should be printed once.
7. Modify to other values ​​and publish again:

```yaml
dd: 2
```

The process prints change events.

## Configuration instructions (example section of config.yaml)

`main.go` will read `polaris.ConfigSourceConfig` from `yggdrasil.example.config_source`:

- `sdk`: Which Polaris SDK to use (corresponds to `yggdrasil.polaris.<name>`)
- `namespace`: The namespace where the configuration file is located (default will fall back to `yggdrasil.InstanceNamespace()`)
- `fileGroup` / `fileName`: Configuration file positioning
- `subscribe`: Whether to subscribe to changes (example is true)
- `fetchTimeout`: pull timeout

The monitored key is specified by `yggdrasil.example.watched_key`, the example is `dd`:
- Note: What is monitored here is the "key in the configuration content", not the file name/group of Polaris.

## FAQ

- no output:
- Confirm that the configuration file has been "published" in the Polaris console (merely saving it as unpublished will usually not be distributed).
- Confirm that the configuration file content contains the key `dd`, and the value is an integer (`Value().Int()` in the example).
- Connection failed:
- Make sure `config_addresses` points to `8093`, not `8091`.
- If authentication is enabled, confirm that the token has permission to read the configuration center.
