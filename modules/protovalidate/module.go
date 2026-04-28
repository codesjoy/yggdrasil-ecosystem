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

package protovalidate

import (
	"context"
	"sync"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
)

const capabilityName = "protovalidate"

type protovalidateModule struct {
	mu       sync.RWMutex
	settings settings
}

type settings struct {
	Default Config `mapstructure:"default"`
}

// Module returns the Yggdrasil v3 Protovalidate capability module.
func Module() module.Module {
	return &protovalidateModule{}
}

// WithModule registers the Protovalidate capability module on a Yggdrasil v3 app.
func WithModule() yggdrasil.Option {
	return yggdrasil.WithModules(Module())
}

func (m *protovalidateModule) Name() string { return capabilityName }

func (m *protovalidateModule) ConfigPath() string { return "yggdrasil.protovalidate" }

func (m *protovalidateModule) Init(_ context.Context, view config.View) error {
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

func (m *protovalidateModule) Capabilities() []module.Capability {
	return []module.Capability{
		capabilities.ProvideOrdered(
			capabilities.UnaryServerInterceptorSpec,
			capabilityName,
			interceptor.NewUnaryServerInterceptorProvider(
				capabilityName,
				func() interceptor.UnaryServerInterceptor {
					return unaryServerInterceptor(
						newValidatorResolver(nil, m.defaultConfig),
					)
				},
			),
		),
		capabilities.ProvideOrdered(
			capabilities.StreamServerInterceptorSpec,
			capabilityName,
			interceptor.NewStreamServerInterceptorProvider(
				capabilityName,
				func() interceptor.StreamServerInterceptor {
					return streamServerInterceptor(
						newValidatorResolver(nil, m.defaultConfig),
					)
				},
			),
		),
	}
}

func (m *protovalidateModule) defaultConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.Default
}
