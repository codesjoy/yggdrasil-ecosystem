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
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
)

const registryAppName = "github.com.codesjoy.yggdrasil-ecosystem.modules.etcd.examples.registry.server"

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

func main() {
	if err := run(); err != nil {
		slog.Error("run registry example", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	app, err := yapp.New(
		registryAppName,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(etcd.Module()),
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop registry example", slog.Any("error", err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := app.Prepare(ctx); err != nil {
		return err
	}

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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()

	inst := demoInstance{
		namespace: "default",
		name:      "example-registry-server",
		version:   "1.0.0",
		region:    "local",
		zone:      "dev",
		campus:    "workspace",
		metadata: map[string]string{
			"env":     "dev",
			"started": time.Now().Format(time.RFC3339),
		},
		endpoints: []yregistry.Endpoint{
			demoEndpoint{scheme: "grpc", address: ln.Addr().String()},
			demoEndpoint{scheme: "http", address: ln.Addr().String()},
		},
	}

	if err := reg.Register(ctx, inst); err != nil {
		return err
	}
	defer func() {
		_ = reg.Deregister(context.Background(), inst)
	}()

	slog.Info("registered etcd example instance", slog.String("address", ln.Addr().String()))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	return nil
}
