package exampleapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

const (
	// SampleService is the single-service demo target.
	SampleService = "sample"
	// LibraryService is the first multi-service demo target.
	LibraryService = "library"
	// GreeterService is the second multi-service demo target.
	GreeterService = "greeter"
)

type greeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	greeting string
	instance string
}

func (s *greeterService) SayHello(
	_ context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	message := s.greeting
	if s.instance != "" {
		message = fmt.Sprintf("%s from %s", message, s.instance)
	}
	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("%s, %s", message, req.GetName()),
	}, nil
}

type exampleConfig struct {
	Service  string            `mapstructure:"service"`
	Services []string          `mapstructure:"services"`
	Greeting string            `mapstructure:"greeting"`
	Instance string            `mapstructure:"instance"`
	Requests int               `mapstructure:"requests"`
	Name     string            `mapstructure:"name"`
	Headers  map[string]string `mapstructure:"headers"`
}

// RunServer starts one v3 RPC server for the given service name.
func RunServer(appName, defaultService, defaultGreeting string) {
	configPath := configPathFromArgs()
	err := yggdrasil.Run(
		context.Background(),
		appName,
		func(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
			cfg, err := loadServerConfig(rt.Config(), defaultService, defaultGreeting)
			if err != nil {
				return nil, err
			}

			return &yggdrasil.BusinessBundle{
				RPCBindings: []yggdrasil.RPCBinding{{
					ServiceName: cfg.Service,
					Desc:        &helloworldpb.GreeterServiceServiceDesc,
					Impl: &greeterService{
						greeting: cfg.Greeting,
						instance: cfg.Instance,
					},
				}},
			}, nil
		},
		yggdrasil.WithConfigPath(configPath),
	)
	if err != nil {
		slog.Error(
			"Run xDS example server",
			slog.String("app", appName),
			slog.String("config", configPath),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
}

// RunClient calls SayHello on each configured service through the xDS resolver and balancer.
func RunClient(appName string, defaultServiceNames ...string) {
	configPath := configPathFromArgs()
	app, err := yapp.New(
		appName,
		yapp.WithConfigPath(configPath),
		yapp.WithModules(xds.Module()),
	)
	if err != nil {
		slog.Error(
			"Create xDS example client app",
			slog.String("app", appName),
			slog.String("config", configPath),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("Stop xDS example client app", slog.Any("error", err))
		}
	}()

	if err := app.Prepare(context.Background()); err != nil {
		slog.Error("Prepare xDS example client app", slog.Any("error", err))
		os.Exit(1)
	}

	cfg, err := loadClientConfig(app.Runtime().Config(), defaultServiceNames)
	if err != nil {
		slog.Error("Load xDS example client config", slog.Any("error", err))
		os.Exit(1)
	}

	for _, serviceName := range cfg.Services {
		if err := callService(app, serviceName, cfg); err != nil {
			slog.Error(
				"Call xDS example service",
				slog.String("service", serviceName),
				slog.Any("error", err),
			)
			return
		}
	}
}

func callService(app *yapp.App, serviceName string, cfg exampleConfig) error {
	summary := make(map[string]int)
	for idx := 0; idx < cfg.Requests; idx++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if len(cfg.Headers) > 0 {
			ctx = metadata.WithOutContext(ctx, metadata.New(cfg.Headers))
		}
		resp, err := invokeSayHello(ctx, app, serviceName, fmt.Sprintf("%s-%d", cfg.Name, idx+1))
		cancel()
		if err != nil {
			return err
		}
		summary[resp]++
	}

	slog.Info(
		"xDS example summary",
		slog.String("service", serviceName),
		slog.Any("responses", summary),
	)
	return nil
}

func invokeSayHello(
	ctx context.Context,
	app *yapp.App,
	serviceName, requestName string,
) (string, error) {
	cli, err := app.NewClient(ctx, serviceName)
	if err != nil {
		return "", fmt.Errorf("create client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	client := helloworldpb.NewGreeterServiceClient(cli)
	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{Name: requestName})
	if err != nil {
		return "", fmt.Errorf("say hello: %w", err)
	}
	slog.Info(
		"SayHello response",
		slog.String("service", serviceName),
		slog.String("message", resp.GetMessage()),
		slog.String("request", requestName),
	)
	return resp.GetMessage(), nil
}

func configPathFromArgs() string {
	if len(os.Args) > 1 && os.Args[1] != "" {
		return os.Args[1]
	}
	return "config.yaml"
}

func loadServerConfig(
	manager *config.Manager,
	defaultService, defaultGreeting string,
) (exampleConfig, error) {
	cfg := exampleConfig{
		Service:  defaultService,
		Greeting: defaultGreeting,
		Headers:  map[string]string{},
	}
	if manager == nil {
		return cfg, nil
	}
	if err := manager.Section("app", "example").Decode(&cfg); err != nil {
		return exampleConfig{}, err
	}
	if cfg.Service == "" {
		cfg.Service = defaultService
	}
	if cfg.Greeting == "" {
		cfg.Greeting = defaultGreeting
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
	return cfg, nil
}

func loadClientConfig(manager *config.Manager, defaultServices []string) (exampleConfig, error) {
	cfg := exampleConfig{
		Services: append([]string(nil), defaultServices...),
		Requests: 1,
		Name:     "xds",
		Headers:  map[string]string{},
	}
	if manager == nil {
		return cfg, nil
	}
	if err := manager.Section("app", "example").Decode(&cfg); err != nil {
		return exampleConfig{}, err
	}
	if len(cfg.Services) == 0 {
		cfg.Services = append([]string(nil), defaultServices...)
	}
	if cfg.Requests < 1 {
		cfg.Requests = 1
	}
	if cfg.Name == "" {
		cfg.Name = "xds"
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
	return cfg, nil
}
