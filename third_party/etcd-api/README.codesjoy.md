# codesjoy etcd API compatibility fork

This module is a minimal compatibility fork of `go.etcd.io/etcd/api/v3` at
`v3.6.8`.

The upstream generated `authpb/auth.pb.go` registers its descriptor as the
unqualified path `auth.proto`. Polaris Specification registers another,
unrelated file under the same path, so binaries that load both SDKs panic
during package initialization.

This fork regenerates only `authpb/auth.pb.go` from the canonical input path
`etcd/api/authpb/auth.proto`. The generated descriptor is therefore registered
as `etcd/api/authpb/auth.proto`. Message names, fields, Go APIs, and wire formats
remain unchanged.

## Consumer replacement

Go module replacement directives are not transitive. Applications that use
the Yggdrasil Polaris and etcd modules in the same binary must add this exact
replacement to their root `go.mod`:

```go
replace go.etcd.io/etcd/api/v3 v3.6.8 => github.com/codesjoy/yggdrasil-ecosystem/third_party/etcd-api/v3 v3.6.8-codesjoy.1
```

## Regeneration

The patched file was generated with the same tool versions used by etcd
`v3.6.8`:

- `protoc v3.20.3`
- `github.com/gogo/protobuf/protoc-gen-gofast v1.3.2`

Run protoc from a directory where the source is available at the canonical
path and preserve source-relative output:

```bash
protoc \
  --proto_path=. \
  --proto_path="$GOGOPROTO_ROOT" \
  --proto_path="$GOGOPROTO_ROOT/protobuf" \
  --gofast_out=paths=source_relative:. \
  etcd/api/authpb/auth.proto
```

The upstream license is retained in [LICENSE](./LICENSE).
