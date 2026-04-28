package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

const (
	appName      = "github.com.codesjoy.yggdrasil-ecosystem.modules.k8s.examples.resolver"
	resolverName = "kubernetes"
	serviceName  = "downstream-service"
)

type statePrinter struct {
	ch chan yresolver.State
}

func main() {
	if err := run(); err != nil {
		slog.Error("run resolver example", slog.Any("error", err))
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
		return fmt.Errorf("create resolver example app: %w", err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop resolver example app", slog.Any("error", err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.Prepare(ctx); err != nil {
		return fmt.Errorf("prepare resolver example app: %w", err)
	}

	snapshot := app.Snapshot()
	if snapshot == nil {
		return fmt.Errorf("runtime snapshot is not ready")
	}

	resolver, err := snapshot.NewResolver(resolverName)
	if err != nil {
		return fmt.Errorf("build resolver %q: %w", resolverName, err)
	}
	if resolver == nil {
		return fmt.Errorf("resolver %q is not configured", resolverName)
	}

	printer := &statePrinter{ch: make(chan yresolver.State, 8)}
	if err := resolver.AddWatch(serviceName, printer); err != nil {
		return fmt.Errorf("add resolver watch for %q: %w", serviceName, err)
	}
	defer func() {
		if err := resolver.DelWatch(serviceName, printer); err != nil {
			slog.Warn("remove resolver watch", slog.Any("error", err))
		}
	}()

	slog.Info(
		"watching resolver updates, press Ctrl+C to exit",
		slog.String("resolver", resolverName),
		slog.String("service", serviceName),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case state := <-printer.ch:
			printState(serviceName, state)
		case <-sigCh:
			return nil
		}
	}
}

func (p *statePrinter) UpdateState(state yresolver.State) {
	select {
	case p.ch <- state:
	default:
	}
}

func printState(service string, state yresolver.State) {
	endpoints := append([]yresolver.Endpoint(nil), state.GetEndpoints()...)
	sort.Slice(endpoints, func(i int, j int) bool {
		if endpoints[i].GetProtocol() == endpoints[j].GetProtocol() {
			return endpoints[i].GetAddress() < endpoints[j].GetAddress()
		}
		return endpoints[i].GetProtocol() < endpoints[j].GetProtocol()
	})

	fmt.Printf("resolver update for %s: %d endpoint(s)\n", service, len(endpoints))
	if len(endpoints) == 0 {
		fmt.Println("- no endpoints available")
		return
	}

	for _, endpoint := range endpoints {
		attrs := endpoint.GetAttributes()
		fmt.Printf(
			"- %s://%s node=%s zone=%s\n",
			endpoint.GetProtocol(),
			endpoint.GetAddress(),
			attrString(attrs["nodeName"]),
			attrString(attrs["zone"]),
		)
	}
}

func attrString(value any) string {
	if value == nil {
		return "-"
	}
	text := fmt.Sprint(value)
	if text == "" {
		return "-"
	}
	return text
}
