# ConfigMap configuration source example

This example demonstrates how to use the ConfigMap configuration source function of k8s to read configuration from Kubernetes ConfigMap and support configuration hot update.

## What you get

- Load configuration from Kubernetes ConfigMap
- Automatically parse YAML/JSON/TOML format (automatically identify based on file extension)
- Support configuration hot update and monitor ConfigMap changes
- Manage multiple configuration sources by priority
- Real-time demonstration configuration change notification

## Preconditions

1. Have an accessible Kubernetes cluster
2. A ConfigMap containing the configuration has been deployed in the cluster
3. Go 1.19+ installed locally
4. kubectl has configured cluster access permissions

### Quickly start local Kubernetes

If you don't have a Kubernetes cluster, you can use one of the following tools to get started quickly:

**Using Minikube:**
```bash
minikube start
```

**Use Kind:**
```bash
kind create cluster
```

**Using Docker Desktop:**
```bash
# Docker Desktop has Kubernetes built-in, just enable it in the settings
```

Verify that the Kubernetes cluster is functioning properly:
```bash
kubectl cluster-info
kubectl get nodes
```

## Startup method

### 1. Create ConfigMap

First create a ConfigMap containing the configuration in Kubernetes:

```bash
kubectl create configmap example-config --from-literal=config.yaml='message: "Hello from ConfigMap!"' -n default
```

Or create it using a YAML file:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-config
  namespace: default
data:
  config.yaml: |
    message: "Hello from ConfigMap!"
    database:
      host: localhost
      port: 5432
    cache:
      enabled: true
      ttl: 3600
```

Create ConfigMap:
```bash
kubectl apply -f configmap.yaml
```

### 2. Verify ConfigMap

View the ConfigMap contents:
```bash
kubectl get configmap example-config -n default -o yaml
```

### 3. Modify configuration (optional)

If the Kubernetes cluster address is not the default, please modify the Namespace and Name configuration in [main.go](main.go):

```go
src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "your-namespace",
    Name:      "your-config",
    Key:       "config.yaml",
    Watch:     true,
    Priority:  source.PriorityRemote,
})
```

Or modify it through environment variables (if you use environment variables to read the configuration):

```bash
export KUBERNETES_NAMESPACE=your-namespace
```

### 4. Run the example

```bash
cd integrations/k8s/example/config-source
go run main.go
```

The program will read the configuration from the ConfigMap and listen for changes.

## Expected output

```
watching config changes, press Ctrl+C to exit...
config changed: type=1, version=1
message: Hello from ConfigMap!
```

## Test configuration update

### Method 1: Update using kubectl

```bash
kubectl patch configmap example-config -n default --type='json' -p='{"data":{"config.yaml":"message: \"Updated message!\nversion: 2.0"}}'
```

Or replace directly:
```bash
kubectl create configmap example-config --from-literal=config.yaml='message: "Updated message!"' --dry-run=client -o yaml | kubectl apply -f -
```

### Method 2: Update using YAML file

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-config
  namespace: default
data:
  config.yaml: |
    message: "Updated from YAML file!"
    version: "2.0"
    settings:
      debug: true
      maxConnections: 100
```

```bash
kubectl apply -f updated-configmap.yaml
```

Observe the program output and you should see notifications of configuration changes:
```
config changed: type=2, version=2
message: Updated from YAML file!
```

### Method 3: Use kubectl edit

```bash
kubectl edit configmap example-config -n default
```

Save after editing and the program will automatically detect the changes.

## Configuration instructions

### ConfigSourceConfig parameters

| Parameters | Type | Description | Default value |
|------|------|------|--------|
| `Namespace` | string | Kubernetes namespace | From environment variable `KUBERNETES_NAMESPACE` or `default` |
| `Name` | string | ConfigMap/Secret name | Required |
| `Key` | string | key in ConfigMap | required |
| `MergeAllKey` | bool | Whether to merge all keys | `false` |
| `Format` | Parser | Configure parser | Automatically infer based on file extension |
| `Priority` | Priority | Configure priority | `source.PriorityRemote` |
| `Watch` | bool | Whether to monitor configuration changes | `false` |
| `Kubeconfig` | string | kubeconfig file path | empty (use in-cluster config) |

### Working mode

**Single Key mode (default):**
```go
src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Watch:     true,
})
```
- Only read the `config.yaml` key in ConfigMap
- Automatically use YAML parser based on `.yaml` extension
-Support hot update

**Multiple Key merge mode:**
```go
src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    MergeAllKey: true,
    Watch:     true,
})
```
- Read all keys in ConfigMap
- Combine all keys into one map
- No format parsing, direct injection as map

### Configuration analysis

Automatically select a parser based on Key's extension:

| extensions | parsers | examples |
|--------|--------|------|
| `.json` | JSON | `config.json` |
| `.yaml`, `.yml` | YAML | `config.yaml` |
| `.toml` | TOML | `config.toml` |
| Other | YAML (default) | `config` |

### Code structure description

```go
// 1. Create a ConfigMap configuration source
src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "example-config",
    Key:       "config.yaml",
    Watch:     true,
    Priority:  source.PriorityRemote,
})
if err != nil {
    panic(err)
}

// 2. Load the configuration source into the framework
if err := config.LoadSource(src); err != nil {
    panic(err)
}

// 3. Add configuration change listener
if err := config.AddWatcher("example", func(ev config.WatchEvent) {
    fmt.Printf("config changed: type=%v, version=%d\n", ev.Type(), ev.Version())

    if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
        var cfg struct {
            Message string `mapstructure:"message"`
        }
        if err := ev.Value().Scan(&cfg); err != nil {
            fmt.Printf("failed to scan config: %v\n", err)
            return
        }
        fmt.Printf("message: %s\n", cfg.Message)
    }
}); err != nil {
    panic(err)
}

// 4. Wait for signal
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
<-sig
```

## Advanced usage

### Multiple configuration source priorities

```go
// 1. Load local configuration (low priority)
localSrc, _ := source.NewFileSource("config.local.yaml", source.PriorityLocal)
config.LoadSource(localSrc)

// 2. Load ConfigMap configuration (high priority, overwrite local configuration)
configMapSrc, _ := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote,
})
config.LoadSource(configMapSrc)

// The configuration of ConfigMap will overwrite the local configuration field with the same name.
```

### Nested configuration

YAML configuration in ConfigMap supports nested structures:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: 30s
    database:
      host: "db.example.com"
      port: 5432
      name: "mydb"
      pool:
        min: 5
        max: 20
    cache:
      type: "redis"
      host: "cache.example.com"
      port: 6379
      ttl: 300
```

Reading in the code:
```go
var cfg struct {
    Server   struct {
        Host    string `mapstructure:"host"`
        Port    int    `mapstructure:"port"`
        Timeout string `mapstructure:"timeout"`
    } `mapstructure:"server"`
    Database struct {
        Host  string `mapstructure:"host"`
        Port  int    `mapstructure:"port"`
        Name  string `mapstructure:"name"`
        Pool  struct {
            Min int `mapstructure:"min"`
            Max int `mapstructure:"max"`
        } `mapstructure:"pool"`
    } `mapstructure:"database"`
    Cache struct {
        Type string `mapstructure:"type"`
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
        TTL  int    `mapstructure:"ttl"`
    } `mapstructure:"cache"`
}
config.Get("app").Scan(&cfg)
```

### Array configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: array-config
data:
  config.yaml: |
    servers:
      - host: "10.0.0.1"
        port: 8080
      - host: "10.0.0.2"
        port: 8080
      - host: "10.0.0.3"
        port: 8080
```

Reading in the code:
```go
var cfg struct {
    Servers []struct {
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
    } `mapstructure:"servers"`
}
config.Get("app").Scan(&cfg)
for _, server := range cfg.Servers {
    fmt.Printf("server: %s:%d\n", server.Host, server.Port)
}
```

### Multiple environment configuration

Use different ConfigMap to manage the configuration of different environments:

```go
// Select different ConfigMap based on environment variables
env := os.Getenv("APP_ENV")
if env == "" {
    env = "dev"
}

configMapName := fmt.Sprintf("app-config-%s", env)

src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      configMapName,
    Key:       "config.yaml",
    Watch:     true,
})
```

## RBAC permissions

Programs require the following RBAC permissions to access the ConfigMap:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: configmap-reader
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: configmap-reader
  namespace: default
subjects:
- kind: ServiceAccount
  name: your-serviceaccount
roleRef:
  kind: Role
  name: configmap-reader
  apiGroup: rbac.authorization.k8s.io
```

## FAQ

**Q: Why is the configuration not read? **

A: Check the following points:
1. Confirm that ConfigMap exists: `kubectl get configmap example-config -n default`
2. Confirm that the Namespace is configured correctly
3. Confirm that the Key configuration is correct
4. Check RBAC permissions

**Q: Didn’t receive notification after configuration update? **

A: Possible reasons:
1. The `Watch` parameter is set to `false`
2. The content of ConfigMap has not changed (the same content will not trigger an update)
3. Kubernetes cluster network issues
4. Watcher channel is full and updates are discarded

**Q: How to handle a large number of configurations? **

A: Recommended method:
1. Use multiple ConfigMap and split them by modules
2. Use `MergeAllKey` mode to merge multiple keys
3. Consider using Secrets to store sensitive configurations

**Q: What is the size limit of ConfigMap? **

A: ConfigMap size limit is 1 MiB (1,048,576 bytes). If the configuration is too large, it is recommended:
1. Split into multiple ConfigMap
2. Use an external configuration center (such as etcd)
3. Store large files in Volume referenced in ConfigMap

**Q: How to use different kubeconfig files? **

A: Set `Kubeconfig` parameters:
```go
src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Kubeconfig: "/path/to/kubeconfig.yaml",
    // ...
})
```

**Q: Does it support configuration encryption? **

A: ConfigMap itself is not encrypted. If encryption is required:
1. Use Secret to store sensitive information (but Secret is only Base64 encoded)
2. Use Kubernetes encryption capabilities (such as KMS encryption)
3. Decrypt the configuration at the application layer
4. Use a key management service (such as HashiCorp Vault)

## Best Practices

1. **Use environment isolation**: Different environments use different ConfigMap (such as `app-config-dev`, `app-config-prod`)
2. **Configuration layering**: Divide the configuration into basic configuration, environment configuration, and instance configuration, and load them according to priority.
3. **Configuration Verification**: Verify the legality of the configuration in the code
4. **Configuration Monitoring**: Record configuration change logs to facilitate auditing and troubleshooting
5. **Configuration version management**: Include version information in the configuration to facilitate rollback
6. **Sensitive information**: Sensitive information (passwords, keys, etc.) should be stored using Secret
7. **Configuration hot update**: Properly configure `Watch` to achieve configuration hot update
8. **RBAC Permissions**: Configure RBAC using the principle of least privilege

## Related documents

- [k8s main documentation](../../README.md)
- [Secret configuration source example](../secret-source/)
- [Resolver example](../resolver/)
- [yggdrasil configuration document](../../../../docs/config.md)

## quit

Press `Ctrl+C` to exit the program.
