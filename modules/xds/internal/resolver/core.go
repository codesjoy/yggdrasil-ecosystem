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

package resolver

import (
	"context"
	"sync"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

type xdsResolver struct {
	cfg      Config
	core     *xdsCore
	watchers map[string]map[yresolver.Client]struct{}
}

type adsSubscriptionClient interface {
	Start() error
	UpdateSubscriptions(lds, rds, cds, eds []string)
	Close()
}

type xdsCore struct {
	cfg       Config
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	apps      map[string]*appInfo
	listeners map[string]*xdsresource.ListenerSnapshot
	routes    map[string]*xdsresource.RouteSnapshot
	clusters  map[string]*xdsresource.ClusterSnapshot
	endpoints map[string]*xdsresource.EDSSnapshot
	onUpdate  func(string, yresolver.State)
	ads       adsSubscriptionClient
}

type appInfo struct {
	listeners map[string]bool
}

var adsClientFactory = func(
	cfg Config,
	handle func(xdsresource.DiscoveryEvent),
) (adsSubscriptionClient, error) {
	return newADSClient(cfg, handle)
}

// NewResolver creates a new xDS resolver.
func NewResolver(_ string, cfg Config) (yresolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())

	core := &xdsCore{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		apps:      make(map[string]*appInfo),
		listeners: make(map[string]*xdsresource.ListenerSnapshot),
		routes:    make(map[string]*xdsresource.RouteSnapshot),
		clusters:  make(map[string]*xdsresource.ClusterSnapshot),
		endpoints: make(map[string]*xdsresource.EDSSnapshot),
	}

	instance := &xdsResolver{
		cfg:      cfg,
		core:     core,
		watchers: make(map[string]map[yresolver.Client]struct{}),
	}
	core.onUpdate = instance.notifyWatchers
	return instance, nil
}

// Provider returns the xDS v3 resolver provider.
func Provider(load ConfigLoader) yresolver.Provider {
	if load == nil {
		load = func(string) Config { return defaultResolverConfig() }
	}
	return yresolver.NewProvider("xds", func(name string) (yresolver.Resolver, error) {
		return NewResolver(name, load(name))
	})
}

func (r *xdsResolver) Type() string {
	return "xds"
}

func (r *xdsResolver) AddWatch(target string, client yresolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	if r.watchers[target] == nil {
		r.watchers[target] = make(map[yresolver.Client]struct{})
	}
	r.watchers[target][client] = struct{}{}

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

func (r *xdsResolver) DelWatch(target string, client yresolver.Client) error {
	r.core.mu.Lock()
	defer r.core.mu.Unlock()

	if watchers := r.watchers[target]; watchers != nil {
		delete(watchers, client)
		if len(watchers) > 0 {
			return nil
		}
		delete(r.watchers, target)
	}

	delete(r.core.apps, target)
	r.core.reconcileSubscriptions()
	if len(r.watchers) == 0 && r.core.ads != nil {
		r.core.ads.Close()
		r.core.ads = nil
	}

	return nil
}

func (r *xdsResolver) notifyWatchers(target string, state yresolver.State) {
	for watcher := range r.watchers[target] {
		watcher.UpdateState(state)
	}
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
	}
	c.apps[target] = app
	return app
}

func (c *xdsCore) handleDiscoveryEvent(event xdsresource.DiscoveryEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch event.Typ {
	case xdsresource.ListenerAdded:
		c.listeners[event.Name] = event.Data.(*xdsresource.ListenerSnapshot)
	case xdsresource.RouteAdded:
		c.routes[event.Name] = event.Data.(*xdsresource.RouteSnapshot)
	case xdsresource.ClusterAdded:
		c.clusters[event.Name] = event.Data.(*xdsresource.ClusterSnapshot)
	case xdsresource.EndpointAdded:
		c.endpoints[event.Name] = event.Data.(*xdsresource.EDSSnapshot)
	}

	c.reconcileSubscriptions()
	c.notifyApps()
}
