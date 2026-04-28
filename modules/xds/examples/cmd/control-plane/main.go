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

package main

import (
	"flag"
	"log/slog"
	"os"

	controlplane "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/controlplane"
)

func main() {
	bootstrapPath := flag.String("bootstrap", "", "path to the control-plane bootstrap config")
	snapshotPath := flag.String("snapshot", "", "path to the xDS snapshot config")
	flag.Parse()

	if *bootstrapPath == "" || *snapshotPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := controlplane.Run(*bootstrapPath, *snapshotPath); err != nil {
		slog.Error("Run xDS control plane", "error", err)
		os.Exit(1)
	}
}
