# etcd example

This directory contains complete examples of the etcd contrib module, showing how to use etcd as a configuration center, registry, and service discovery.

## Quick Start

### Preconditions

1. Install etcd (Docker is recommended)
2. Install Go 1.19+

### Start etcd

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

Verify that etcd is running properly:
```bash
etcdctl --endpoints=127.0.0.1:2379 endpoint health
```

## Example list

| Example | Description | Difficulty | Estimated time |
|------|------|--------|----------|
| [allinone](allinone/) | Full example: configuration source + registry + service discovery | ⭐⭐ | 15 minutes |
| [config-source/blob](config-source/blob/) | Configure source blob mode | ⭐ | 10 minutes |
| [config-source/kv](config-source/kv/) | Configure source kv mode | ⭐⭐ | 15 minutes |
| [registry](registry/) | Service Registration Center | ⭐⭐ | 10 minutes |
| [resolver](resolver/) | Service discovery | ⭐⭐ | 10 minutes |

## Learning path

### Path 1: Get started quickly (recommended for novices)

1. **[allinone](allinone/)**: Run the complete example to understand the overall process
2. **[config-source/blob](config-source/blob/)**: Learn the basic usage of configuration sources
3. **[registry](registry/)**: Learn the basic usage of service registration
4. **[resolver](resolver/)**: Learn the basic usage of service discovery

Estimated time: 45 minutes

### Path 2: In-depth learning (recommended for experienced developers)

1. **[allinone](allinone/)**: Understand the complete process
2. **[config-source/blob](config-source/blob/)**: Master blob mode
3. **[config-source/kv](config-source/kv/)**: Master kv mode
4. **[registry](registry/)**: In-depth understanding of the registration center
5. **[resolver](resolver/)**: In-depth understanding of service discovery

Estimated time: 60 minutes

### Path 3: Learn on demand

Choose the corresponding example according to your needs:

- Requires configuration management → [config-source/blob](config-source/blob/) or [config-source/kv](config-source/kv/)
- Requires service registration → [registry](registry/)
- Requires service discovery → [resolver](resolver/)
- Requires full functionality → [allinone](allinone/)

## Example comparison

### Configuration source example

| Features | blob mode | kv mode |
|------|-----------|---------|
| Storage | Single key | Multiple keys |
| Suitable for scenarios | The configuration is large and updates are infrequent | The configuration is small and fine-grained updates are required |
| Update granularity | Overall update | A configuration item can be updated individually |
| Example | [config-source/blob](config-source/blob/) | [config-source/kv](config-source/kv/) |

### Registration center and service discovery

| Components | Features | Examples |
|------|------|------|
| Registration center | Register service instances to etcd and maintain heartbeats | [registry](registry/) |
| Service discovery | Discover service instances from etcd and listen for changes | [resolver](resolver/) |
| Combined use | Use the registry and service discovery at the same time | [allinone](allinone/) |

## Run the example

### Basic steps

The running steps of each example are basically the same:

```bash
# 1. Enter the sample directory
cd integrations/etcd/example/<example-name>

# 2. Modify configuration (if necessary)
vim config.yaml

# 3. Run the example
go run <main-file>
```

### Run multiple examples simultaneously

You can run multiple examples simultaneously to observe their interactions:

```bash
# Terminal 1: Start the registry example
cd integrations/etcd/example/registry
go run server.go

# Terminal 2: Start Service Discovery Example
cd integrations/etcd/example/resolver
go run client.go
```

## FAQ

### Q: What should I do if the sample fails to run?

A: Check the following points:
1. Confirm etcd is running: `etcdctl --endpoints=127.0.0.1:2379 endpoint health`
2. Confirm that the port is not occupied: `netstat -an | grep 2379`
3. Check the error log and troubleshoot based on the error information.

### Q: How to modify etcd address?

A: Modify the `config.yaml` file in the example directory:

```yaml
etcd:
  client:
    endpoints:
      - "your-etcd-endpoint:2379"
```

### Q: Does the example require dependencies?

A: Most examples only require the etcd client library, which will be downloaded automatically. Make sure the network connection is normal before running.

### Q: How to stop the sample?

A: Press `Ctrl+C` and the example will exit gracefully.

## Advanced usage

### Modify configuration

You can dynamically modify the configuration in etcd while the example is running:

```bash
# View current configuration
etcdctl --endpoints=127.0.0.1:2379 get /config/app --prefix

# Update configuration
etcdctl --endpoints=127.0.0.1:2379 put /config/app "new config"
```

### Monitor etcd

Use `etcdctl` to monitor the status of etcd:

```bash
# View all keys
etcdctl --endpoints=127.0.0.1:2379 get "" --prefix --keys-only

# View member status
etcdctl --endpoints=127.0.0.1:2379 member list

# View cluster health status
etcdctl --endpoints=127.0.0.1:2379 endpoint health --cluster
```

### Clean test data

After the test is completed, clean the test data in etcd:

```bash
# Delete configuration
etcdctl --endpoints=127.0.0.1:2379 del /config --prefix

# Delete registration information
etcdctl --endpoints=127.0.0.1:2379 del /yggdrasil/registry --prefix

# Delete all data (use with caution)
etcdctl --endpoints=127.0.0.1:2379 del "" --prefix
```

## Architecture diagram

```
┌─────────────────────────────────────────────────────────┐
│                   Application                      │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │ Config      │  │ Registry    │  │ Resolver    │ │
│  │ Source      │  │             │  │             │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘ │
│         │                │                │          │
└─────────┼────────────────┼────────────────┼──────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────────────────────────────────────────┐
│                    etcd                          │
│                                                     │
│ /config/app (configuration source) │
│ /registry/... (registry/discovery) │
└─────────────────────────────────────────────────────┘
```

## Production environment recommendations

### etcd cluster

It is recommended to use etcd cluster (3 or 5 nodes) for production environment:

```yaml
etcd:
  client:
    endpoints:
      - "etcd-1:2379"
      - "etcd-2:2379"
      - "etcd-3:2379"
```

### TLS encryption

It is recommended to enable TLS encryption for production environments:

```yaml
etcd:
  client:
    tls:
      certFile: "/path/to/cert.pem"
      keyFile: "/path/to/key.pem"
      caFile: "/path/to/ca.pem"
```

### Authentication and authorization

It is recommended to enable authentication in production environment:

```yaml
etcd:
  client:
    username: "your-username"
    password: "your-password"
```

### Monitoring alarms

It is recommended to monitor the following indicators:
- etcd connection status
- Configure update frequency
- Number of registrations/de-registrations
- Service discovery delays
- Changes in the number of instances

## Reference documentation

- [etcd main documentation](../README.md)
- [etcd official documentation](https://etcd.io/docs/latest/)
- [Yggdrasil Framework Documentation](../../README.md)
- [Other contrib modules](../../)

## Feedback and Contribution

If you encounter problems during use or have suggestions for improvements, please welcome:

1. Submit an Issue
2. Submit Pull Request
3. Discuss in the community

## License

This example is licensed under the Yggdrasil Project.
