package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	helloworldpb "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/helloworld"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/examples/multi-instance/internal/names"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
)

func main() {
	app, err := yapp.New(
		names.ClientApp,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(polaris.Module()),
	)
	if err != nil {
		slog.Error("create multi-instance client app", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop multi-instance client app", slog.Any("error", err))
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
	for i := 1; i <= 8; i++ {
		callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := client.SayHello(callCtx, &helloworldpb.SayHelloRequest{
			Name: fmt.Sprintf("request-%d", i),
		})
		callCancel()
		if err != nil {
			slog.Error("call SayHello", slog.Int("request", i), slog.Any("error", err))
			os.Exit(1)
		}
		fmt.Println(resp.GetMessage())
		time.Sleep(200 * time.Millisecond)
	}
}
