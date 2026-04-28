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
	"log/slog"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3"
	yapp "github.com/codesjoy/yggdrasil/v3/app"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	kvAppName = "github.com.codesjoy.yggdrasil-ecosystem.modules.etcd.examples.config-source.kv"
	kvPrefix  = "/examples/etcd/kv"
)

type kvConfig struct {
	Greeting string `mapstructure:"greeting"`
	Name     string `mapstructure:"name"`
}

func main() {
	if err := runKV(); err != nil {
		slog.Error("run kv example", slog.Any("error", err))
		os.Exit(1)
	}
}

func runKV() error {
	if err := seedKVConfig(); err != nil {
		return err
	}

	app, err := yapp.New(
		kvAppName,
		yapp.WithConfigPath("config.yaml"),
		yapp.WithModules(etcd.Module()),
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop kv example app", slog.Any("error", err))
		}
	}()

	if _, err := app.Compose(context.Background(), composeKV); err != nil {
		return err
	}
	return nil
}

func composeKV(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	cfg := kvConfig{}
	if err := rt.Config().Section("app", "config_source").Decode(&cfg); err != nil {
		return nil, err
	}
	fmt.Printf("%s, %s\n", cfg.Greeting, cfg.Name)
	return &yapp.BusinessBundle{}, nil
}

func seedKVConfig() error {
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
	ops := []clientv3.Op{
		clientv3.OpPut(kvPrefix+"/app/config_source/greeting", "hello from etcd kv"),
		clientv3.OpPut(kvPrefix+"/app/config_source/name", "kv"),
	}
	_, err = cli.Txn(ctx).Then(ops...).Commit()
	return err
}
