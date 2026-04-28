package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/metadata-discovery/internal/names"
	"github.com/codesjoy/yggdrasil/v3"
)

type serverConfig struct {
	Greeting string `mapstructure:"greeting"`
	Version  string `mapstructure:"version"`
}

type greeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	greeting string
	version  string
}

func (s *greeterService) SayHello(
	_ context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("%s [%s] to %s", s.greeting, s.version, req.GetName()),
	}, nil
}

func main() {
	configPath := "config-stable.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	err := yggdrasil.Run(
		context.Background(),
		names.ServerApp,
		compose,
		yggdrasil.WithConfigPath(configPath),
		polaris.WithModule(),
	)
	if err != nil {
		slog.Error(
			"run metadata-discovery server",
			slog.String("config", configPath),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
}

func compose(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
	cfg := serverConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "metadata_discovery").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Greeting == "" {
		cfg.Greeting = "hello from metadata discovery"
	}
	if cfg.Version == "" {
		cfg.Version = "unknown"
	}

	rt.Logger().Info("compose metadata-discovery bundle", "version", cfg.Version)

	return &yggdrasil.BusinessBundle{
		RPCBindings: []yggdrasil.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl: &greeterService{
				greeting: cfg.Greeting,
				version:  cfg.Version,
			},
		}},
		Diagnostics: []yggdrasil.BundleDiag{{
			Code:    "metadata_discovery.version",
			Message: cfg.Version,
		}},
	}, nil
}
