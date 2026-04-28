package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
)

const appName = "github.com.codesjoy.yggdrasil-ecosystem.modules.k8s.examples.config-source"

type appConfig struct {
	Greeting string `mapstructure:"greeting"`
	Name     string `mapstructure:"name"`
}

func main() {
	if err := run(); err != nil {
		slog.Error("run config-source example", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	app, err := yapp.New(
		appName,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(k8s.Module()),
	)
	if err != nil {
		return fmt.Errorf("create config-source example app: %w", err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop config-source example app", slog.Any("error", err))
		}
	}()

	if _, err := app.Compose(context.Background(), compose); err != nil {
		return fmt.Errorf("compose config-source example: %w", err)
	}
	return nil
}

func compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	if rt == nil || rt.Config() == nil {
		return nil, fmt.Errorf("runtime config is not ready")
	}

	cfg := appConfig{}
	if err := rt.Config().Section("app", "config_source").Decode(&cfg); err != nil {
		return nil, err
	}

	fmt.Printf("%s, %s\n", cfg.Greeting, cfg.Name)
	return &yapp.BusinessBundle{}, nil
}
