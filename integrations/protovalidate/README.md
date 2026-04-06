# Protovalidate Integration for Yggdrasil

## Overview

This module integrates Buf Protovalidate with Yggdrasil as inbound server-side
request validation.

It provides:

1. Side-effect registration of Yggdrasil interceptors named `protovalidate`.
2. Explicit APIs for manual validation and manual interceptor wiring.

Default scope in v1:

- Unary server requests.
- Stream server inbound messages.
- Config-driven default validator behavior for side-effect registered interceptors.

## Installation

```bash
go get github.com/codesjoy/yggdrasil-ecosystem/integrations/protovalidate/v2
```

Enable interceptors via side-effect import:

```go
import _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/protovalidate/v2"
```

## Quick Start

### 1. Annotate your protobuf schema

```proto
syntax = "proto3";

package example.user.v1;

import "buf/validate/validate.proto";

message CreateUserRequest {
  string email = 1 [(buf.validate.field).string.email = true];
}
```

### 2. Enable the interceptors in Yggdrasil config

```yaml
yggdrasil:
  protovalidate:
    default:
      failFast: true

  interceptor:
    unary_server:
      - protovalidate
    stream_server:
      - protovalidate
```

### 3. Register the integration

```go
import (
    _ "github.com/codesjoy/yggdrasil-ecosystem/integrations/protovalidate/v2"
    "github.com/codesjoy/yggdrasil/v2"
)
```

Invalid protobuf requests are rejected with `INVALID_ARGUMENT`, and
Protovalidate violation details are attached to the returned Yggdrasil status.

## Configuration

### Default validator config (`yggdrasil.protovalidate.<name>`)

The side-effect registered interceptors read defaults from
`yggdrasil.protovalidate.default`.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `failFast` | `bool` | `false` | Stop validation on the first rule violation instead of accumulating all violations |

Notes:

- This config only affects the default validator used by side-effect registered
  interceptors.
- Explicit `New()` and package-level `Validate()` remain code-driven and do not
  implicitly read config.
- `disableLazy` is intentionally not exposed through config because it requires
  pre-warmed message descriptors to be safe.

## Explicit API

Use the package directly when you want to validate messages outside the default
interceptor registration flow:

```go
import (
    yggvalidate "github.com/codesjoy/yggdrasil-ecosystem/integrations/protovalidate/v2"
)

validator, err := yggvalidate.New()
if err != nil {
    panic(err)
}

if err := validator.Validate(req); err != nil {
    return err
}
```

You can also wire interceptors manually:

```go
validator, err := yggvalidate.New()
if err != nil {
    panic(err)
}

unary := yggvalidate.UnaryServerInterceptor(validator)
stream := yggvalidate.StreamServerInterceptor(validator)

_ = unary
_ = stream
```

## Public API

- `New(options ...bufprotovalidate.ValidatorOption) (Validator, error)`
- `Validate(msg proto.Message, options ...bufprotovalidate.ValidationOption) error`
- `UnaryServerInterceptor(validator Validator) interceptor.UnaryServerInterceptor`
- `StreamServerInterceptor(validator Validator) interceptor.StreamServerInterceptor`

## Limits

- v1 validates inbound server traffic only.
- v1 does not add client-side validation.
- Only `failFast` is exposed as a config-driven default.
- Behavior follows Protovalidate defaults unless you create and inject a custom
  validator yourself.
