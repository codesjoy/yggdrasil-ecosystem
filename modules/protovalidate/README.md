# Protovalidate Module for Yggdrasil v3

This module integrates Buf Protovalidate with Yggdrasil v3 through the
module/capability runtime. It validates inbound server-side protobuf requests.

It provides:

- `protovalidate.Module()` for unary and stream server interceptor capabilities.
- `protovalidate.WithModule()` as a convenience Yggdrasil option.
- Explicit APIs for manual validation and manual interceptor wiring.

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/modules/protovalidate/v3
```

Register the module explicitly:

```go
app, err := yggdrasil.New(
    "example",
    protovalidate.WithModule(),
)
```

Blank-import side-effect registration is not supported in v3.

## Configuration

Enable the interceptor names in the Yggdrasil v3 server config:

```yaml
yggdrasil:
  protovalidate:
    default:
      failFast: true

  server:
    interceptors:
      unary:
        - protovalidate
      stream:
        - protovalidate
```

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `failFast` | `bool` | `false` | Stop validation on the first rule violation instead of accumulating all violations |

## Explicit API

Use the package directly when you want to validate messages outside module
registration:

```go
validator, err := protovalidate.New()
if err != nil {
    panic(err)
}

if err := validator.Validate(req); err != nil {
    return err
}
```

You can also wire interceptors manually:

```go
validator, err := protovalidate.New()
if err != nil {
    panic(err)
}

unary := protovalidate.UnaryServerInterceptor(validator)
stream := protovalidate.StreamServerInterceptor(validator)

_ = unary
_ = stream
```

Invalid protobuf requests are rejected with `INVALID_ARGUMENT`, and
Protovalidate violation details are attached to the returned Yggdrasil status.

## Examples

Runnable examples are available under [`examples/`](./examples/):

- `manual-validation` shows direct `protovalidate.New` and
  `protovalidate.Validate` usage.
- `quickstart` shows Yggdrasil unary and stream server interceptors rejecting
  invalid requests before business handlers run.

## Limits

- Validates inbound server traffic only.
- Does not add client-side validation.
- Only `failFast` is exposed as a config-driven default.
- Behavior follows Protovalidate defaults unless you create and inject a custom
  validator yourself.
