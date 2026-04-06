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

import "github.com/codesjoy/yggdrasil/v2/config"

// Config defines config-driven defaults for the Protovalidate integration.
type Config struct {
	FailFast bool `json:"failFast" mapstructure:"failFast" toml:"failFast" yaml:"failFast"`
}

// LoadConfig loads Protovalidate config for the given name from Yggdrasil config.
func LoadConfig(name string) Config {
	var cfg Config
	_ = config.Get(config.Join(config.KeyBase, "protovalidate", name)).Scan(&cfg)
	return cfg
}
