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

package etcd

import (
	"context"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type etcdClient interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Put(
		ctx context.Context,
		key, val string,
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

type realEtcdClient struct {
	*clientv3.Client
}

func newClient(cfg ClientConfig) (*clientv3.Client, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"127.0.0.1:2379"}
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	return clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
}

func wrapClient(cli *clientv3.Client) etcdClient {
	if cli == nil {
		return nil
	}
	return &realEtcdClient{Client: cli}
}
