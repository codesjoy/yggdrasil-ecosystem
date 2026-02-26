# Multi SDK

This scenario is for demonstration: select the Polaris SDKContext (distinguished by name) through the `sdk` field to use multiple sets of Polaris configurations in the same process.

Example configuration snippet:

```yaml
yggdrasil:
  polaris:
    blue:
      config_file: "./polaris-blue.yaml"
      token: "token-blue"
    green:
      addresses:
        - "127.0.0.1:8091"
      config_addresses:
        - "127.0.0.1:8093"
      token: "token-green"

  registry:
    type: polaris
    config:
      sdk: blue
      namespace: "default"

  resolver:
    green:
      type: polaris
      config:
        sdk: green
        namespace: "default"
```
