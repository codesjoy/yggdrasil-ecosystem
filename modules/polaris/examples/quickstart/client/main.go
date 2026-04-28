package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/quickstart/internal/names"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
)

func main() {
	app, err := yapp.New(
		names.ClientApp,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(polaris.Module()),
	)
	if err != nil {
		slog.Error("create client app", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop client app", slog.Any("error", err))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := app.NewClient(ctx, names.ServerApp)
	if err != nil {
		slog.Error("create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = cli.Close() }()

	client := helloworldpb.NewGreeterServiceClient(cli)
	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{Name: "quickstart"})
	if err != nil {
		slog.Error("call SayHello", slog.Any("error", err))
		os.Exit(1)
	}

	fmt.Println(resp.GetMessage())
}
