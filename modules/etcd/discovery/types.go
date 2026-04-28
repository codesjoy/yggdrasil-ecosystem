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

import "github.com/mitchellh/mapstructure"

type instanceRecord struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Region    string            `json:"region"`
	Zone      string            `json:"zone"`
	Campus    string            `json:"campus"`
	Metadata  map[string]string `json:"metadata"`
	Endpoints []endpointRecord  `json:"endpoints"`
}

type endpointRecord struct {
	Scheme   string            `json:"scheme"`
	Address  string            `json:"address"`
	Metadata map[string]string `json:"metadata"`
}

func decodeMap(input map[string]any, target any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:  target,
		TagName: "mapstructure",
	})
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}
