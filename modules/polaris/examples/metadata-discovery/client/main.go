package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/metadata-discovery/internal/names"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
)

func main() {
	configPath := "config-stable.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	app, err := yapp.New(
		names.ClientApp,
		yapp.WithConfigPath(configPath),
		yapp.WithModules(polaris.Module()),
	)
	if err != nil {
		slog.Error(
			"create metadata-discovery client app",
			slog.String("config", configPath),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop metadata-discovery client app", slog.Any("error", err))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cli, err := app.NewClient(ctx, names.ServerApp)
	if err != nil {
		slog.Error("create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = cli.Close() }()

	time.Sleep(2 * time.Second)

	client := helloworldpb.NewGreeterServiceClient(cli)
	for i := 1; i <= 4; i++ {
		callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := client.SayHello(callCtx, &helloworldpb.SayHelloRequest{
			Name: fmt.Sprintf("metadata-%d", i),
		})
		callCancel()
		if err != nil {
			slog.Error("call SayHello", slog.Int("request", i), slog.Any("error", err))
			os.Exit(1)
		}
		fmt.Println(resp.GetMessage())
	}
}
