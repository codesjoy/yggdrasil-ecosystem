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
	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/discovery"
	internalresolver "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resolver"
)

type settings struct {
	XDS       map[string]xdsProfile `mapstructure:"xds"`
	Discovery struct {
		Resolvers map[string]resolverSpec `mapstructure:"resolvers"`
	} `mapstructure:"discovery"`
}

type xdsProfile struct {
	Config map[string]any `mapstructure:"config"`
}

type resolverSpec struct {
	Type   string             `mapstructure:"type"`
	Config resolverProfileRef `mapstructure:"config"`
}

type resolverProfileRef struct {
	Name string `mapstructure:"name"`
}

func (m *xdsModule) resolverConfig(name string) discovery.ResolverConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profileName := m.settings.Discovery.Resolvers[name].Config.Name
	if profileName == "" {
		profileName = "default"
	}
	return internalresolver.DecodeConfig(m.settings.XDS[profileName].Config)
}
