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
	"fmt"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientFactory lazily creates and caches Kubernetes clients per kubeconfig path.
type ClientFactory struct {
	mu      sync.Mutex
	clients map[string]kubernetes.Interface
}

// NewClientFactory creates a Kubernetes client factory.
func NewClientFactory() *ClientFactory {
	return &ClientFactory{
		clients: map[string]kubernetes.Interface{},
	}
}

// Client returns a cached client for the provided kubeconfig path.
func (f *ClientFactory) Client(kubeconfigPath string) (kubernetes.Interface, error) {
	key := strings.TrimSpace(kubeconfigPath)

	f.mu.Lock()
	if client := f.clients[key]; client != nil {
		f.mu.Unlock()
		return client, nil
	}
	f.mu.Unlock()

	cfg, err := buildConfig(key)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	f.mu.Lock()
	if existing := f.clients[key]; existing != nil {
		f.mu.Unlock()
		return existing, nil
	}
	f.clients[key] = client
	f.mu.Unlock()
	return client, nil
}

func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	return rest.InClusterConfig()
}
