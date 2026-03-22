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

package xds

import (
	"context"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/resolver"
)

func init() {
	resolver.RegisterBuilder("xds", func(name string) (resolver.Resolver, error) {
		cfg := LoadResolverConfig(name)
		return NewResolver(name, cfg)
	})
}

type xdsResolver struct {
	cfg  ResolverConfig
	core *xdsCore
}

type adsSubscriptionClient interface {
	Start() error
	UpdateSubscriptions(lds, rds, cds, eds []string)
}

type xdsCore struct {
	cfg          ResolverConfig
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	apps         map[string]*appInfo
	listeners    map[string]*listenerSnapshot
	routes       map[string]*routeSnapshot
	clusters     map[string]*clusterSnapshot
	endpoints    map[string]*edsSnapshot
	onUpdate     func(string, resolver.State)
	ads          adsSubscriptionClient
	dependencies map[string]*resourceDependencies
}

type appInfo struct {
	listeners map[string]bool
	clusters  map[string]bool
}

type resourceDependencies struct {
	routes   map[string]bool
	clusters map[string]bool
}

var adsClientFactory = func(
	cfg ResolverConfig,
	handle func(discoveryEvent),
) (adsSubscriptionClient, error) {
	return newADSClient(cfg, handle)
}

// NewResolver creates a new xDS resolver.
func NewResolver(_ string, cfg ResolverConfig) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())

	core := &xdsCore{
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		apps:         make(map[string]*appInfo),
		listeners:    make(map[string]*listenerSnapshot),
		routes:       make(map[string]*routeSnapshot),
		clusters:     make(map[string]*clusterSnapshot),
		endpoints:    make(map[string]*edsSnapshot),
		dependencies: make(map[string]*resourceDependencies),
	}

	return &xdsResolver{
		cfg:  cfg,
		core: core,
	}, nil
}

func (r *xdsResolver) Type() string {
	return "xds"
}

func (r *xdsResolver) AddWatch(target string, client resolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	if r.core.onUpdate != nil {
		r.core.onUpdate = func(_ string, state resolver.State) {
			client.UpdateState(state)
		}
	}

	app := r.core.ensureAppLocked(target)
	app.listeners[r.listenerName(target)] = true

	if r.core.ads == nil {
		ads, err := adsClientFactory(r.core.cfg, r.core.handleDiscoveryEvent)
		if err != nil {
			return err
		}
		if err := ads.Start(); err != nil {
			return err
		}
		r.core.ads = ads
	}

	r.core.reconcileSubscriptions()
	return nil
}

func (r *xdsResolver) DelWatch(target string, _ resolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	delete(r.core.apps, target)
	r.core.reconcileSubscriptions()
	return nil
}

func (r *xdsResolver) listenerName(target string) string {
	if listenerName, ok := r.cfg.ServiceMap[target]; ok {
		return listenerName
	}
	return target
}

func (c *xdsCore) ensureAppLocked(target string) *appInfo {
	if app, ok := c.apps[target]; ok {
		return app
	}

	app := &appInfo{
		listeners: make(map[string]bool),
		clusters:  make(map[string]bool),
	}
	c.apps[target] = app
	return app
}
