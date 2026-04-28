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

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ResolverConfig configures the etcd resolver provider.
type ResolverConfig struct {
	Client    string        `mapstructure:"client"`
	Prefix    string        `mapstructure:"prefix"`
	Namespace string        `mapstructure:"namespace"`
	Protocols []string      `mapstructure:"protocols"`
	Debounce  time.Duration `mapstructure:"debounce"`
}

// ResolverConfigLoader loads resolver config for one named resolver.
type ResolverConfigLoader func(name string) ResolverConfig

// NormalizeConfig fills defaults into the etcd resolver config.
func NormalizeConfig(cfg ResolverConfig) ResolverConfig {
	if cfg.Prefix == "" {
		cfg.Prefix = "/yggdrasil/registry"
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if len(cfg.Protocols) == 0 {
		cfg.Protocols = []string{"grpc", "http"}
	}
	return cfg
}

// Resolver is the etcd-backed discovery resolver.
type Resolver struct {
	name   string
	cfg    ResolverConfig
	cli    *clientv3.Client
	client internalclient.Client

	mu       sync.Mutex
	watchers map[string]map[yresolver.Client]struct{}
	cancels  map[string]context.CancelFunc
}

// NewResolver creates one etcd-backed resolver.
func NewResolver(name string, cfg ResolverConfig) (*Resolver, error) {
	cfg = NormalizeConfig(cfg)
	clientCfg := internalclient.LoadConfig(cfg.Client)
	cli, err := internalclient.New(clientCfg)
	if err != nil {
		return nil, err
	}
	return &Resolver{
		name:     name,
		cfg:      cfg,
		cli:      cli,
		client:   internalclient.Wrap(cli),
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}, nil
}

// ResolverProvider returns the v3 etcd resolver provider.
func ResolverProvider(load ResolverConfigLoader) yresolver.Provider {
	if load == nil {
		load = func(string) ResolverConfig { return ResolverConfig{} }
	}
	return yresolver.NewProvider("etcd", func(name string) (yresolver.Resolver, error) {
		return NewResolver(name, load(name))
	})
}

// Type returns the resolver backend type.
func (r *Resolver) Type() string { return "etcd" }

// AddWatch starts watching one service name.
func (r *Resolver) AddWatch(serviceName string, watcher yresolver.Client) error {
	r.mu.Lock()
	ws := r.watchers[serviceName]
	if ws == nil {
		ws = map[yresolver.Client]struct{}{}
		r.watchers[serviceName] = ws
	}
	ws[watcher] = struct{}{}
	_, running := r.cancels[serviceName]
	if !running {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancels[serviceName] = cancel
		go r.watchLoop(ctx, serviceName)
	}
	r.mu.Unlock()
	return nil
}

// DelWatch stops watching one service name.
func (r *Resolver) DelWatch(serviceName string, watcher yresolver.Client) error {
	r.mu.Lock()
	ws := r.watchers[serviceName]
	if ws != nil {
		delete(ws, watcher)
		if len(ws) == 0 {
			delete(r.watchers, serviceName)
			if cancel, ok := r.cancels[serviceName]; ok {
				delete(r.cancels, serviceName)
				cancel()
			}
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Resolver) watchLoop(ctx context.Context, serviceName string) {
	debounce := r.cfg.Debounce
	kick := make(chan struct{}, 1)

	notify := func() {
		select {
		case kick <- struct{}{}:
		default:
		}
	}

	notify()
	go func() {
		defer close(kick)
		prefix := r.servicePrefix(serviceName)
		var revision int64
		for {
			getResp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
			if err == nil {
				revision = getResp.Header.Revision
			}
			watchCh := r.client.Watch(
				ctx,
				prefix,
				clientv3.WithPrefix(),
				clientv3.WithRev(revision+1),
			)
			for resp := range watchCh {
				if resp.Canceled {
					return
				}
				notify()
			}
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()

	var timer *time.Timer
	for {
		if debounce > 0 && timer == nil {
			timer = time.NewTimer(time.Hour)
			if !timer.Stop() {
				<-timer.C
			}
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case _, ok := <-kick:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			if debounce <= 0 {
				r.fetchAndNotify(ctx, serviceName)
				continue
			}
			timer.Reset(debounce)
		case <-timerChan(timer, debounce):
			r.fetchAndNotify(ctx, serviceName)
		}
	}
}

func timerChan(timer *time.Timer, debounce time.Duration) <-chan time.Time {
	if debounce <= 0 || timer == nil {
		return nil
	}
	return timer.C
}

func (r *Resolver) fetchAndNotify(ctx context.Context, serviceName string) {
	state, err := r.fetchState(ctx, serviceName)
	if err != nil {
		return
	}
	for _, watcher := range r.snapshotWatchers(serviceName) {
		watcher.UpdateState(state)
	}
}

func (r *Resolver) snapshotWatchers(serviceName string) []yresolver.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := r.watchers[serviceName]
	out := make([]yresolver.Client, 0, len(ws))
	for watcher := range ws {
		out = append(out, watcher)
	}
	return out
}

func (r *Resolver) fetchState(ctx context.Context, serviceName string) (yresolver.State, error) {
	resp, err := r.client.Get(ctx, r.servicePrefix(serviceName), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	state := yresolver.BaseState{
		Attributes: map[string]any{
			"service":   serviceName,
			"namespace": r.cfg.Namespace,
			"revision":  resp.Header.Revision,
		},
		Endpoints: make([]yresolver.Endpoint, 0),
	}

	allow := toProtocolAllow(r.cfg.Protocols)
	instances := map[string]struct{}{}

	for _, item := range resp.Kvs {
		var rec instanceRecord
		if err := json.Unmarshal(item.Value, &rec); err != nil {
			continue
		}
		if rec.Name != serviceName || rec.Namespace != r.cfg.Namespace {
			continue
		}

		for _, endpoint := range rec.Endpoints {
			if !allow[endpoint.Scheme] {
				continue
			}
			key := instanceKey(
				rec.Namespace,
				rec.Name,
				rec.Version,
				endpoint.Scheme,
				endpoint.Address,
			)
			if _, ok := instances[key]; ok {
				continue
			}
			instances[key] = struct{}{}

			attrs := map[string]any{
				"instance_version": rec.Version,
				"instance_region":  rec.Region,
				"instance_zone":    rec.Zone,
				"instance_campus":  rec.Campus,
			}
			for name, value := range rec.Metadata {
				attrs[name] = value
			}
			for name, value := range endpoint.Metadata {
				attrs[name] = value
			}
			state.Endpoints = append(state.Endpoints, yresolver.BaseEndpoint{
				Address:    endpoint.Address,
				Protocol:   endpoint.Scheme,
				Attributes: attrs,
			})
		}
	}

	sort.Slice(state.Endpoints, func(i int, j int) bool {
		return state.Endpoints[i].Name() < state.Endpoints[j].Name()
	})
	return state, nil
}

func (r *Resolver) servicePrefix(serviceName string) string {
	return strings.Join([]string{r.cfg.Prefix, r.cfg.Namespace, serviceName}, "/")
}

func instanceKey(
	namespace string,
	name string,
	version string,
	scheme string,
	address string,
) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", namespace, name, version, scheme, address)
}

func toProtocolAllow(list []string) map[string]bool {
	if len(list) == 0 {
		list = []string{"grpc", "http"}
	}
	out := make(map[string]bool, len(list))
	for _, item := range list {
		out[item] = true
	}
	return out
}
