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

package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildConfigFromKubeconfig(t *testing.T) {
	path := writeTestKubeconfig(t)

	cfg, err := buildConfig(path)
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}
	if cfg.Host != "https://127.0.0.1:6443" {
		t.Fatalf("buildConfig Host = %q, want https://127.0.0.1:6443", cfg.Host)
	}
}

func TestBuildConfigWithoutKubeconfigExecutesInClusterPath(t *testing.T) {
	cfg, err := buildConfig("")
	if cfg == nil && err == nil {
		t.Fatal("buildConfig(\"\") returned nil config and nil error")
	}
}

func TestClientFactoryCachesClients(t *testing.T) {
	path := writeTestKubeconfig(t)
	factory := NewClientFactory()

	first, err := factory.Client("  " + path + "  ")
	if err != nil {
		t.Fatalf("Client() first call error = %v", err)
	}
	second, err := factory.Client(path)
	if err != nil {
		t.Fatalf("Client() second call error = %v", err)
	}
	if first != second {
		t.Fatal("Client() did not reuse cached client")
	}
	if len(factory.clients) != 1 {
		t.Fatalf("cached clients = %d, want 1", len(factory.clients))
	}
}

func TestClientFactoryBuildConfigError(t *testing.T) {
	factory := NewClientFactory()
	missing := filepath.Join(t.TempDir(), "missing-kubeconfig")

	if _, err := factory.Client(missing); err == nil ||
		!strings.Contains(err.Error(), "failed to build kubernetes config") {
		t.Fatalf("Client() error = %v, want build config error", err)
	}
}

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kubeconfig")
	content := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    token: test-token
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
