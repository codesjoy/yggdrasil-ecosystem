module github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples

go 1.25.7

replace github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3 => ../

require (
	github.com/codesjoy/pkg/basic/xerror v0.0.0-20260225033528-924cf61d0622
	github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3 v3.0.0-00010101000000-000000000000
	github.com/codesjoy/yggdrasil/v3 v3.0.0-rc.2
	github.com/envoyproxy/go-control-plane v0.14.0
	github.com/envoyproxy/go-control-plane/envoy v1.36.0
	github.com/fsnotify/fsnotify v1.9.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260122232226-8e98ce8d340d
	google.golang.org/grpc v1.80.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20251210132809-ee656c7534f5 // indirect
	github.com/codesjoy/pkg/utils v0.0.0-20260227125603-faf7bfdf00a7 // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/envoyproxy/go-control-plane/ratelimit v0.1.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/go-chi/chi/v5 v5.2.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.39.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260122232226-8e98ce8d340d // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
