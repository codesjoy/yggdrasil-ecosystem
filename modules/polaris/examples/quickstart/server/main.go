package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/quickstart/internal/names"
	"github.com/codesjoy/yggdrasil/v3"
)

type quickstartConfig struct {
	Greeting string `mapstructure:"greeting"`
}

type greeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	greeting string
}

func (s *greeterService) SayHello(
	_ context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("%s, %s", s.greeting, req.GetName()),
	}, nil
}

func main() {
	err := yggdrasil.Run(
		context.Background(),
		names.ServerApp,
		compose,
		yggdrasil.WithConfigPath("config.yaml"),
		polaris.WithModule(),
	)
	if err != nil {
		slog.Error("run quickstart server", slog.Any("error", err))
		os.Exit(1)
	}
}

// Compose installs the smallest end-to-end bundle used by the quickstart path.
func compose(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
	cfg := quickstartConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "quickstart").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Greeting == "" {
		cfg.Greeting = "hello from quickstart"
	}

	rt.Logger().Info("compose quickstart bundle", "greeting", cfg.Greeting)

	return &yggdrasil.BusinessBundle{
		RPCBindings: []yggdrasil.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl:        &greeterService{greeting: cfg.Greeting},
		}},
		Diagnostics: []yggdrasil.BundleDiag{{
			Code:    "quickstart.greeting",
			Message: cfg.Greeting,
		}},
	}, nil
}
