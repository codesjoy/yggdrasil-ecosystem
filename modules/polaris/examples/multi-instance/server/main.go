package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/multi-instance/internal/names"
	"github.com/codesjoy/yggdrasil/v3"
)

type serverConfig struct {
	Greeting   string `mapstructure:"greeting"`
	InstanceID string `mapstructure:"instance_id"`
}

type greeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	greeting   string
	instanceID string
}

func (s *greeterService) SayHello(
	_ context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("%s from %s to %s", s.greeting, s.instanceID, req.GetName()),
	}, nil
}

func main() {
	configPath := "config-a.yaml"
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
			"run multi-instance server",
			slog.String("config", configPath),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
}

func compose(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
	cfg := serverConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "multi_instance").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Greeting == "" {
		cfg.Greeting = "hello from multi-instance"
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = "unknown"
	}

	rt.Logger().Info("compose multi-instance bundle", "instance_id", cfg.InstanceID)

	return &yggdrasil.BusinessBundle{
		RPCBindings: []yggdrasil.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl: &greeterService{
				greeting:   cfg.Greeting,
				instanceID: cfg.InstanceID,
			},
		}},
		Diagnostics: []yggdrasil.BundleDiag{{
			Code:    "multi_instance.instance",
			Message: cfg.InstanceID,
		}},
	}, nil
}
