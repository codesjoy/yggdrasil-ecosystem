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
	// nolint:gosec
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// RegistryConfig configures the etcd registry provider.
type RegistryConfig struct {
	Client        string        `mapstructure:"client"`
	Prefix        string        `mapstructure:"prefix"`
	TTL           time.Duration `mapstructure:"ttl"`
	KeepAlive     *bool         `mapstructure:"keep_alive"`
	RetryInterval time.Duration `mapstructure:"retry_interval"`
}

// Registry is the etcd-backed service registry.
type Registry struct {
	cfg    RegistryConfig
	cli    *clientv3.Client
	client internalclient.Client

	mu    sync.Mutex
	regs  map[string]registryEntry
	close chan struct{}
	once  sync.Once
	after func(time.Duration) <-chan time.Time
}

type registryEntry struct {
	cancel context.CancelFunc
	lease  clientv3.LeaseID
}

// NewRegistry creates one etcd-backed registry.
func NewRegistry(cfg RegistryConfig) (*Registry, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "/yggdrasil/registry"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 10 * time.Second
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 3 * time.Second
	}
	clientCfg := internalclient.LoadConfig(cfg.Client)
	cli, err := internalclient.New(clientCfg)
	if err != nil {
		return nil, err
	}
	return &Registry{
		cfg:    cfg,
		cli:    cli,
		client: internalclient.Wrap(cli),
		regs:   map[string]registryEntry{},
		close:  make(chan struct{}),
		after:  time.After,
	}, nil
}

// NewRegistryFromMap creates one registry from the resolved v3 config map.
func NewRegistryFromMap(cfgMap map[string]any) (yregistry.Registry, error) {
	var cfg RegistryConfig
	if err := decodeMap(cfgMap, &cfg); err != nil {
		return nil, err
	}
	return NewRegistry(cfg)
}

// RegistryProvider returns the v3 etcd registry provider.
func RegistryProvider() yregistry.Provider {
	return yregistry.NewProvider("etcd", NewRegistryFromMap)
}

// Type returns the registry backend type.
func (r *Registry) Type() string { return "etcd" }

// Register adds one service instance to etcd.
func (r *Registry) Register(ctx context.Context, inst yregistry.Instance) error {
	if inst == nil {
		return errors.New("nil instance")
	}
	key, value, err := r.buildKeyValue(inst)
	if err != nil {
		return err
	}
	keepAlive := r.cfg.KeepAlive == nil || *r.cfg.KeepAlive

	r.mu.Lock()
	ent, ok := r.regs[key]
	if ok && ent.cancel != nil {
		ent.cancel()
		delete(r.regs, key)
	}
	bgCtx, cancel := context.WithCancel(context.Background())
	r.regs[key] = registryEntry{cancel: cancel}
	r.mu.Unlock()

	if err := r.putOnce(ctx, key, value); err != nil {
		cancel()
		r.mu.Lock()
		delete(r.regs, key)
		r.mu.Unlock()
		return err
	}

	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.regs, key)
			r.mu.Unlock()
		}()

		if keepAlive {
			r.keepAliveLoop(bgCtx, key, value)
			return
		}

		select {
		case <-r.close:
			cancel()
		case <-bgCtx.Done():
		}
	}()

	return nil
}

// Deregister removes one service instance from etcd.
func (r *Registry) Deregister(ctx context.Context, inst yregistry.Instance) error {
	if inst == nil {
		return errors.New("nil instance")
	}
	key, _, err := r.buildKeyValue(inst)
	if err != nil {
		return err
	}

	r.mu.Lock()
	ent, ok := r.regs[key]
	if ok && ent.cancel != nil {
		ent.cancel()
		delete(r.regs, key)
	}
	r.mu.Unlock()

	_, err = r.client.Delete(ctx, key)
	return err
}

// Close stops all outstanding keepalive loops and closes the etcd client.
func (r *Registry) Close() error {
	r.once.Do(func() {
		close(r.close)
		r.mu.Lock()
		for _, ent := range r.regs {
			if ent.cancel != nil {
				ent.cancel()
			}
		}
		r.regs = map[string]registryEntry{}
		r.mu.Unlock()
		if r.client != nil {
			_ = r.client.Close()
		}
	})
	return nil
}

func (r *Registry) putOnce(ctx context.Context, key string, value string) error {
	resp, err := r.client.Grant(ctx, int64(r.cfg.TTL/time.Second))
	if err != nil {
		return err
	}
	_, err = r.client.Put(ctx, key, value, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}
	r.mu.Lock()
	ent := r.regs[key]
	ent.lease = resp.ID
	r.regs[key] = ent
	r.mu.Unlock()
	return nil
}

func (r *Registry) keepAliveLoop(ctx context.Context, key string, value string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.close:
			return
		default:
		}

		resp, err := r.client.Grant(ctx, int64(r.cfg.TTL/time.Second))
		if err != nil {
			if !r.waitRetry(ctx) {
				return
			}
			continue
		}

		keepaliveCh, keepaliveErr := r.client.KeepAlive(ctx, resp.ID)
		if keepaliveErr != nil {
			if !r.waitRetry(ctx) {
				return
			}
			continue
		}

		_, err = r.client.Put(ctx, key, value, clientv3.WithLease(resp.ID))
		if err != nil {
			if !r.waitRetry(ctx) {
				return
			}
			continue
		}

		r.mu.Lock()
		ent := r.regs[key]
		ent.lease = resp.ID
		r.regs[key] = ent
		r.mu.Unlock()

	LoopKeepAlive:
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.close:
				return
			case keepaliveResp, ok := <-keepaliveCh:
				if !ok {
					break LoopKeepAlive
				}
				if keepaliveResp.TTL <= 0 {
					break LoopKeepAlive
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-r.close:
			return
		case <-r.retryAfter(r.cfg.RetryInterval):
		}
	}
}

func (r *Registry) retryAfter(delay time.Duration) <-chan time.Time {
	if r.after != nil {
		return r.after(delay)
	}
	return time.After(delay)
}

func (r *Registry) waitRetry(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-r.close:
		return false
	case <-r.retryAfter(r.cfg.RetryInterval):
		return true
	}
}

func (r *Registry) buildKeyValue(inst yregistry.Instance) (string, string, error) {
	endpoints := inst.Endpoints()
	endpointRecords := make([]endpointRecord, len(endpoints))
	for i, endpoint := range endpoints {
		endpointRecords[i] = endpointRecord{
			Scheme:   endpoint.Scheme(),
			Address:  endpoint.Address(),
			Metadata: endpoint.Metadata(),
		}
	}
	record := instanceRecord{
		Namespace: inst.Namespace(),
		Name:      inst.Name(),
		Version:   inst.Version(),
		Region:    inst.Region(),
		Zone:      inst.Zone(),
		Campus:    inst.Campus(),
		Metadata:  inst.Metadata(),
		Endpoints: endpointRecords,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return "", "", err
	}
	hash := sha1.New() // nolint:gosec
	hash.Write(payload)
	sum := hex.EncodeToString(hash.Sum(nil))

	parts := []string{r.cfg.Prefix}
	if inst.Namespace() != "" {
		parts = append(parts, inst.Namespace())
	}
	if inst.Name() != "" {
		parts = append(parts, inst.Name())
	}
	parts = append(parts, sum)
	return strings.Join(parts, "/"), string(payload), nil
}
