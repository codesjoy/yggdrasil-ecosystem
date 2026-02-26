# Secret configuration source example

This example demonstrates how to use the Secret configuration source function of k8s to read configuration from Kubernetes Secret and support configuration hot update.

## What you get

- Load configuration from Kubernetes Secret
- Automatically parse YAML/JSON/TOML format (automatically identify based on file extension)
- Support configuration hot update and monitor Secret changes
- Manage multiple configuration sources by priority
- Suitable for storing sensitive configurations (passwords, keys, etc.)
- Real-time demonstration configuration change notification

## Preconditions

1. Have an accessible Kubernetes cluster
2. The Secret containing the configuration has been deployed in the cluster
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

### 1. Create Secret

First create a Secret containing the configuration in Kubernetes:

```bash
kubectl create secret generic example-secret --from-literal=config.yaml='message: "Hello from Secret!"' -n default
```

Or create it using a YAML file:

```bash
kubectl apply -f secret.yaml
```

**Notice:**
- The value of the `data` field must be Base64 encoded
- The value of the `stringData` field will be automatically Base64 encoded (recommended)
- Do not submit Secrets to version control systems

### 2. Verify Secret

View Secret content:
```bash
kubectl get secret example-secret -n default -o yaml
```

### 3. Modify configuration (optional)

If the Kubernetes cluster address is not the default, please modify the Namespace and Name configuration in [main.go](main.go):

```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "your-namespace",
    Name:      "your-secret",
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
cd integrations/k8s/example/secret-source
go run main.go
```

The program will read the configuration from the Secret and listen for changes.

## Expected output

```
watching secret changes, press Ctrl+C to exit...
config changed: type=1, version=1
message: Hello from Secret!
```

## Test configuration update

### Method 1: Update using kubectl

```bash
kubectl patch secret example-secret -n default --type='json' -p='{"data":{"config.yaml":"bWVzc2FnZTogVXBkYXRlZCBtZXNzYWdlIQp2ZXJzaW9uOiAiMi4w"}}'
```

Or replace directly:
```bash
kubectl create secret generic example-secret --from-literal=config.yaml='message: "Updated message!"' --dry-run=client -o yaml | kubectl apply -f -
```

### Method 2: Update using YAML file

Create the updated Secret YAML file:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: example-secret
  namespace: default
type: Opaque
stringData:
  config.yaml: |
    message: "Updated from YAML file!"
    version: "2.0"
    database:
      host: db.example.com
      port: 5432
      username: admin
      password: new-secret-password
```

Apply updates:
```bash
kubectl apply -f updated-secret.yaml
```

Observe the program output and you should see notifications of configuration changes:
```
config changed: type=2, version=2
message: Updated from YAML file!
```

### Method 3: Use kubectl edit

```bash
kubectl edit secret example-secret -n default
```

Save after editing and the program will automatically detect the changes.

## Configuration instructions

### ConfigSourceConfig parameters

| Parameters | Type | Description | Default value |
|------|------|------|--------|
| `Namespace` | string | Kubernetes namespace | From environment variable `KUBERNETES_NAMESPACE` or `default` |
| `Name` | string | Secret name | Required |
| `Key` | string | key in Secret | required |
| `MergeAllKey` | bool | Whether to merge all keys | `false` |
| `Format` | Parser | Configure parser | Automatically infer based on file extension |
| `Priority` | Priority | Configure priority | `source.PriorityRemote` |
| `Watch` | bool | Whether to monitor configuration changes | `false` |
| `Kubeconfig` | string | kubeconfig file path | empty (use in-cluster config) |

### Working mode

**Single Key mode (default):**
```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-secret",
    Key:       "config.yaml",
    Watch:     true,
})
```
- Only read the `config.yaml` key in Secret
- Automatically use YAML parser based on `.yaml` extension
-Support hot update

**Multiple Key merge mode:**
```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-secret",
    MergeAllKey: true,
    Watch:     true,
})
```
- Read all keys in Secret
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

### The difference between Secret and ConfigMap

| Features | ConfigMap | Secret |
|------|-----------|--------|
| Data storage | Plain text storage | Base64 encoding storage |
| Usage scenarios | Non-sensitive configuration | Sensitive configuration (password, key, etc.) |
| Size limit | 1 MiB | 1 MiB |
| RBAC permissions | `configmaps` permission required | `secrets` permission required |
| Visibility | Any user with permissions can view | Requires specific permissions to view |
| Encryption | Not supported | Not supported (Base64 is not encryption) |

### Code structure description

```go
// 1. Create Secret configuration source
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "example-secret",
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

// 2. Load ConfigMap configuration (medium priority)
configMapSrc, _ := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-config",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote,
})
config.LoadSource(configMapSrc)

// 3. Load Secret configuration (high priority, override other configurations)
secretSrc, _ := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "my-secret",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote + 1,
})
config.LoadSource(secretSrc)

// The configuration of Secret will overwrite the fields of the same name in ConfigMap and local configuration.
```

### Separation of sensitive configurations

Store sensitive configuration and non-sensitive configuration separately:

```yaml
# ConfigMap: non-sensitive configuration
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
    logging:
      level: "info"
---
# Secret: sensitive configuration
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
type: Opaque
stringData:
  config.yaml: |
    database:
      password: "secret-password"
    api:
      apiKey: "api-key-123"
```

Load in code:
```go
// Load ConfigMap
configMapSrc, _ := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "app-config",
    Key:       "config.yaml",
    Priority:  source.PriorityLocal,
})
config.LoadSource(configMapSrc)

// Load Secret
secretSrc, _ := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      "app-secret",
    Key:       "config.yaml",
    Priority:  source.PriorityRemote,
})
config.LoadSource(secretSrc)
```

### Nested configuration

YAML configuration in Secret supports nested structures:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
type: Opaque
stringData:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: 30s
    database:
      host: "db.example.com"
      port: 5432
      name: "mydb"
      username: "admin"
      password: "secret-password"
    cache:
      type: "redis"
      host: "cache.example.com"
      port: 6379
      ttl: 300
    oauth:
      clientId: "client-id-123"
      clientSecret: "client-secret-456"
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
        Host     string `mapstructure:"host"`
        Port     int    `mapstructure:"port"`
        Name     string `mapstructure:"name"`
        Username string `mapstructure:"username"`
        Password string `mapstructure:"password"`
    } `mapstructure:"database"`
    Cache struct {
        Type string `mapstructure:"type"`
        Host string `mapstructure:"host"`
        Port int    `mapstructure:"port"`
        TTL  int    `mapstructure:"ttl"`
    } `mapstructure:"cache"`
    OAuth struct {
        ClientID     string `mapstructure:"clientId"`
        ClientSecret string `mapstructure:"clientSecret"`
    } `mapstructure:"oauth"`
}
config.Get("app").Scan(&cfg)
```

### Array configuration

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: array-secret
type: Opaque
stringData:
  config.yaml: |
    apiKeys:
      - name: "service-a"
        key: "key-123"
      - name: "service-b"
        key: "key-456"
      - name: "service-c"
        key: "key-789"
```

Reading in the code:
```go
var cfg struct {
    APIKeys []struct {
        Name string `mapstructure:"name"`
        Key  string `mapstructure:"key"`
    } `mapstructure:"apiKeys"`
}
config.Get("app").Scan(&cfg)
for _, apiKey := range cfg.APIKeys {
    fmt.Printf("api key: %s - %s\n", apiKey.Name, apiKey.Key)
}
```

### Multiple environment configuration

Use different Secrets to manage the configuration of different environments:

```go
// Select different Secrets based on environment variables
env := os.Getenv("APP_ENV")
if env == "" {
    env = "dev"
}

secretName := fmt.Sprintf("app-secret-%s", env)

src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Namespace: "default",
    Name:      secretName,
    Key:       "config.yaml",
    Watch:     true,
})
```

## RBAC permissions

Programs require the following RBAC permissions to access Secrets:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: secret-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: secret-reader
  namespace: default
subjects:
- kind: ServiceAccount
  name: your-serviceaccount
roleRef:
  kind: Role
  name: secret-reader
  apiGroup: rbac.authorization.k8s.io
```

## Safety precautions

### Secret is not encryption

Secret data is Base64 encoded, not encrypted. Any user with permission to access the Secret can decode and view it.

```bash
# View the Secret content (you can see the Base64 encoded data)
kubectl get secret example-secret -n default -o yaml

# Decode Secret data
echo "bWVzc2FnZTogSGVsbG8gZnJvbSBTZWNyZXQhCg==" | base64 -d
# Output: message: Hello from Secret!
```

### Security Advice

1. **Use Kubernetes Encryption Features**
- Enable etcd encryption (Encryption at Rest)
- Use KMS to encrypt etcd data

2. **Restrict Secret access**
- Use RBAC to limit who can access Secrets
- Use namespaces to isolate Secrets in different environments
- Avoid using ClusterRole to grant Secret permissions

3. **Do not submit the Secret to the version control system**
- Add `*.secret.yaml` to `.gitignore`
- Use `kubectl create secret --dry-run` to generate Secret YAML

4. **Regular rotation of Secrets**
- Regularly update sensitive information such as passwords and keys
- Use Secret management tools (such as HashiCorp Vault)

5. **Use an external key management service**
   - HashiCorp Vault
   - AWS Secrets Manager
   - Azure Key Vault
   - Google Secret Manager

6. **Audit Secret Access**
- Enable Kubernetes audit logs
- Monitor Secret access records

7. **Use temporary credentials**
- For database connections, etc., use short-lived credentials
- Automatically rotate credentials regularly

### Encrypted Secret example

Encrypt the Secret using Kubernetes encryption features:

```yaml
# encryption-config.yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
    - secrets
    providers:
    - aescbc:
        keys:
        - name: key1
          secret: <base64-encoded-key>
    - identity: {}
```

Specify the encryption configuration when starting the API Server:
```bash
kube-apiserver \
  --encryption-provider-config=/etc/kubernetes/encryption-config.yaml
```

## FAQ

**Q: Why is the configuration not read? **

A: Check the following points:
1. Confirm that Secret exists: `kubectl get secret example-secret -n default`
2. Confirm that the Namespace is configured correctly
3. Confirm that the Key configuration is correct
4. Check RBAC permissions

**Q: Didn’t receive notification after configuration update? **

A: Possible reasons:
1. The `Watch` parameter is set to `false`
2. The Secret content has not changed (the same content will not trigger an update)
3. Kubernetes cluster network issues
4. Watcher channel is full and updates are discarded

**Q: How to handle a large number of configurations? **

A: Recommended method:
1. Use multiple Secrets and split them by modules
2. Use `MergeAllKey` mode to merge multiple keys
3. Consider using an external configuration center (such as etcd, Vault)

**Q: What is the Secret size limit? **

A: Secrets are limited to 1 MiB (1,048,576 bytes). If the configuration is too large, it is recommended:
1. Split into multiple Secrets
2. Use external configuration center (such as etcd, Vault)
3. Store large files in the Volume referenced in the Secret

**Q: Is Secret safe? **

A: The security of Secret depends on:
1. **etcd encryption**: Enable Encryption at Rest
2. **RBAC Permissions**: Restrict Secret access permissions
3. **Kubernetes version**: Use the latest version to fix security vulnerabilities
4. **Network Isolation**: Use NetworkPolicy to restrict etcd access

**Q: How to use different kubeconfig files? **

A: Set `Kubeconfig` parameters:
```go
src, err := k8s.NewSecretSource(k8s.ConfigSourceConfig{
    Kubeconfig: "/path/to/kubeconfig.yaml",
    // ...
})
```

**Q: How to decode Secret data? **

A: Use base64 tool:
```bash
# Way 1: Use kubectl
kubectl get secret example-secret -n default -o jsonpath='{.data.config\.yaml}' | base64 -d

# Method 2: Use base64 command
echo "bWVzc2FnZTogSGVsbG8gZnJvbSBTZWNyZXQhCg==" | base64 -d
```

## Best Practices

1. **Use Secret for sensitive information**: Passwords, keys, certificates and other sensitive information must be stored using Secret
2. **Use ConfigMap for non-sensitive information**: Use ConfigMap for normal configuration to reduce the use of Secrets
3. **Configuration layering**: Divide the configuration into basic configuration, environment configuration, and instance configuration, and load them according to priority.
4. **Configuration Verification**: Verify the legality of the configuration in the code
5. **Configuration Monitoring**: Record configuration change logs to facilitate auditing and troubleshooting
6. **Configuration version management**: Include version information in the configuration to facilitate rollback
7. **Enable Encryption**: Enable Kubernetes Encryption at Rest to encrypt etcd data
8. **Principle of Least Privilege**: Use RBAC to restrict Secret access permissions
9. **Periodic rotation**: Regularly update sensitive information such as passwords and keys
10. **Do not submit Secret**: Do not submit Secret to version control system

## Related documents

- [k8s main documentation](../../README.md)
- [ConfigMap configuration source example](../config-source/)
- [Resolver example](../resolver/)
- [yggdrasil configuration document](../../../../docs/config.md)
- [Kubernetes Secret Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)

## quit

Press `Ctrl+C` to exit the program.
