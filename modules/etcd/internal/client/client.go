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

package client

import (
	"context"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// DefaultClientName is the default named etcd client reference.
	DefaultClientName = "default"
)

// Config configures one named etcd client.
type Config struct {
	Endpoints   []string      `mapstructure:"endpoints"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
}

// Client is the minimal etcd client surface used by this module.
type Client interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Put(
		ctx context.Context,
		key string,
		val string,
		opts ...clientv3.OpOption,
	) (*clientv3.PutResponse, error)
	Delete(
		ctx context.Context,
		key string,
		opts ...clientv3.OpOption,
	) (*clientv3.DeleteResponse, error)
	Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)
	KeepAlive(
		ctx context.Context,
		id clientv3.LeaseID,
	) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

type realClient struct {
	*clientv3.Client
}

var (
	configMu     sync.RWMutex
	configLoader = func(string) Config { return Config{} }
)

// ConfigureConfigLoader replaces the named client config loader.
func ConfigureConfigLoader(loader func(string) Config) {
	if loader == nil {
		loader = func(string) Config { return Config{} }
	}
	configMu.Lock()
	configLoader = loader
	configMu.Unlock()
}

// ResolveName resolves one named client reference.
func ResolveName(name string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return DefaultClientName
}

// LoadConfig loads and normalizes one named client config.
func LoadConfig(name string) Config {
	configMu.RLock()
	loader := configLoader
	configMu.RUnlock()
	return Normalize(loader(ResolveName(name)))
}

// Normalize fills defaults into one client config.
func Normalize(cfg Config) Config {
	cfg.Endpoints = append([]string(nil), cfg.Endpoints...)
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"127.0.0.1:2379"}
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	return cfg
}

// New creates one real etcd client from the normalized config.
func New(cfg Config) (*clientv3.Client, error) {
	cfg = Normalize(cfg)
	return clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
}

// Wrap converts one real etcd client into the internal Client interface.
func Wrap(cli *clientv3.Client) Client {
	if cli == nil {
		return nil
	}
	return &realClient{Client: cli}
}
