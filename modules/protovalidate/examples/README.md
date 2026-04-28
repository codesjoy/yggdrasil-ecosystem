# Protovalidate Examples

These examples demonstrate Buf Protovalidate with the Yggdrasil v3
Protovalidate module.

## Scenarios

| Scenario | What it demonstrates |
| --- | --- |
| [manual-validation](./manual-validation/) | Direct `protovalidate.New` and `protovalidate.Validate` usage |
| [quickstart](./quickstart/) | Yggdrasil server-side unary and stream request validation interceptors |

## Generate Protobuf Code

The generated protobuf files are checked in. Regenerate them after changing
`proto/user/v1/user.proto`:

```bash
cd modules/protovalidate/examples
buf generate
```

## Run Manual Validation

```bash
cd modules/protovalidate/examples/manual-validation
go run .
```

Expected output includes:

```text
valid request passed
invalid request violations:
```

## Run Quickstart

Start the server:

```bash
cd modules/protovalidate/examples/quickstart/server
go run .
```

Run the client in another terminal:

```bash
cd modules/protovalidate/examples/quickstart/client
go run .
```

Expected client output includes:

```text
valid unary accepted: user-1 ada@example.com
invalid unary rejected: INVALID_ARGUMENT
invalid stream close rejected: INVALID_ARGUMENT
```

The server config enables both unary and stream server interceptors:

```yaml
yggdrasil:
  server:
    interceptors:
      unary:
        - "protovalidate"
      stream:
        - "protovalidate"
```

Invalid requests are rejected before the business handler runs.
