package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
)

const appName = "github.com.codesjoy.yggdrasil-ecosystem.modules.polaris.examples.config-source.app"

type configSourceConfig struct {
	Greeting string `mapstructure:"greeting"`
	Name     string `mapstructure:"name"`
}

func main() {
	app, err := yapp.New(
		appName,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(polaris.Module()),
	)
	if err != nil {
		slog.Error("create config-source app", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop config-source app", slog.Any("error", err))
		}
	}()

	if _, err := app.Compose(context.Background(), compose); err != nil {
		slog.Error("compose config-source app", slog.Any("error", err))
		os.Exit(1)
	}
}

func compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	if rt == nil || rt.Config() == nil {
		return nil, fmt.Errorf("runtime config is not ready")
	}
	cfg := configSourceConfig{}
	if err := rt.Config().Section("app", "config_source").Decode(&cfg); err != nil {
		return nil, err
	}

	fmt.Printf("%s, %s\n", cfg.Greeting, cfg.Name)
	return &yapp.BusinessBundle{}, nil
}
