// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	allinoneAppName = "github.com.codesjoy.yggdrasil-ecosystem.modules.etcd.examples.allinone"
	allinoneKey     = "/examples/etcd/allinone/config.yaml"
)

type allinoneConfig struct {
	Message string `mapstructure:"message"`
}

type resolverPrinter struct {
	ch chan yresolver.State
}

func (p *resolverPrinter) UpdateState(state yresolver.State) {
	select {
	case p.ch <- state:
	default:
	}
}

func main() {
	if err := seedAllinoneConfig(); err != nil {
		slog.Error("seed allinone config", slog.Any("error", err))
		os.Exit(1)
	}
	if err := runAllinone(); err != nil {
		slog.Error("run allinone example", slog.Any("error", err))
		os.Exit(1)
	}
}

func runAllinone() error {
	app, err := yapp.New(
		allinoneAppName,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(etcd.Module()),
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop allinone example", slog.Any("error", err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := app.Prepare(ctx); err != nil {
		return err
	}

	rt := app.Runtime()
	if rt == nil || rt.Config() == nil {
		return fmt.Errorf("runtime config is not ready")
	}
	cfg := allinoneConfig{}
	if err := rt.Config().Section("app", "allinone").Decode(&cfg); err != nil {
		return err
	}
	fmt.Println(cfg.Message)

	snapshot := app.Snapshot()
	if snapshot == nil {
		return fmt.Errorf("runtime snapshot is not ready")
	}
	reg, err := snapshot.NewRegistry()
	if err != nil {
		return err
	}
	if closer, ok := reg.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}
	res, err := snapshot.NewResolver("default")
	if err != nil {
		return err
	}
	if closer, ok := res.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	inst := demoInstance{
		namespace: "default",
		name:      "example-allinone-service",
		version:   "1.0.0",
		metadata:  map[string]string{"mode": "allinone"},
		endpoints: []yregistry.Endpoint{
			demoEndpoint{scheme: "grpc", address: "127.0.0.1:19001"},
			demoEndpoint{scheme: "http", address: "127.0.0.1:18081"},
		},
	}
	if err := reg.Register(ctx, inst); err != nil {
		return err
	}
	defer func() {
		_ = reg.Deregister(context.Background(), inst)
	}()

	printer := &resolverPrinter{ch: make(chan yresolver.State, 4)}
	if err := res.AddWatch(inst.name, printer); err != nil {
		return err
	}
	defer func() {
		_ = res.DelWatch(inst.name, printer)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case state := <-printer.ch:
			slog.Info("allinone resolver update", slog.Int("endpoints", len(state.GetEndpoints())))
		case <-sigCh:
			return nil
		}
	}
}

func seedAllinoneConfig() error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, allinoneKey, "app:\n  allinone:\n    message: hello from etcd allinone\n")
	return err
}

type demoInstance struct {
	namespace string
	name      string
	version   string
	region    string
	zone      string
	campus    string
	metadata  map[string]string
	endpoints []yregistry.Endpoint
}

func (d demoInstance) Region() string                  { return d.region }
func (d demoInstance) Zone() string                    { return d.zone }
func (d demoInstance) Campus() string                  { return d.campus }
func (d demoInstance) Namespace() string               { return d.namespace }
func (d demoInstance) Name() string                    { return d.name }
func (d demoInstance) Version() string                 { return d.version }
func (d demoInstance) Metadata() map[string]string     { return d.metadata }
func (d demoInstance) Endpoints() []yregistry.Endpoint { return d.endpoints }

type demoEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
}

func (d demoEndpoint) Scheme() string              { return d.scheme }
func (d demoEndpoint) Address() string             { return d.address }
func (d demoEndpoint) Metadata() map[string]string { return d.metadata }
