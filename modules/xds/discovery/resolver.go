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
	internalresolver "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resolver"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
)

// NewResolver creates a new xDS resolver.
func NewResolver(name string, cfg ResolverConfig) (yresolver.Resolver, error) {
	return internalresolver.NewResolver(name, cfg)
}

// ResolverProvider returns the xDS v3 resolver provider.
func ResolverProvider(load ResolverConfigLoader) yresolver.Provider {
	return internalresolver.ResolverProvider(load)
}
