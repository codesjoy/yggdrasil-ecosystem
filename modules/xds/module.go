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

	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/discovery"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/traffic"
	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/module"
)

const capabilityName = "xds"

type xdsModule struct {
	mu       sync.RWMutex
	settings settings
}

// Module returns the Yggdrasil v3 xDS capability module.
func Module() module.Module {
	return &xdsModule{}
}

// WithModule registers the xDS capability module on a Yggdrasil v3 app.
func WithModule() yggdrasil.Option {
	return yggdrasil.WithModules(Module())
}

func (m *xdsModule) Name() string { return capabilityName }

func (m *xdsModule) ConfigPath() string { return "yggdrasil" }

func (m *xdsModule) Init(_ context.Context, view config.View) error {
	var next settings
	if view.Exists() {
		if err := view.Decode(&next); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.settings = next
	m.mu.Unlock()
	return nil
}

func (m *xdsModule) Capabilities() []module.Capability {
	return []module.Capability{
		capabilities.ProvideNamed(
			capabilities.ResolverProviderSpec,
			capabilityName,
			discovery.ResolverProvider(m.resolverConfig),
		),
		capabilities.ProvideNamed(
			capabilities.BalancerProviderSpec,
			capabilityName,
			traffic.BalancerProvider(),
		),
	}
}
