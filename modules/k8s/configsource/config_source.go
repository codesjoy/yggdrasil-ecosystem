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

package configsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3/internal/kube"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	// KindConfigMap is the declarative config source kind for ConfigMaps.
	KindConfigMap = "kubernetes-configmap"
	// KindSecret is the declarative config source kind for Secrets.
	//nolint:gosec // Declarative kind name, not credential material.
	KindSecret = "kubernetes-secret"

	resourceTypeConfigMap = "configmap"
	resourceTypeSecret    = "secret"
)

// Config configures a Kubernetes ConfigMap or Secret source.
type Config struct {
	Namespace    string        `mapstructure:"namespace"`
	Name         string        `mapstructure:"name"`
	Key          string        `mapstructure:"key"`
	MergeAllKeys bool          `mapstructure:"merge_all_keys"`
	Format       source.Parser `mapstructure:"format"`
	Watch        bool          `mapstructure:"watch"`
	Kubeconfig   string        `mapstructure:"kubeconfig"`
}

type configSource struct {
	kind            string
	resourceType    string
	cfg             Config
	watch           bool
	clientForConfig func(string) (kubernetes.Interface, error)

	closeOnce sync.Once
	closeCh   chan struct{}
}

// NewConfigMapSource creates a ConfigMap-backed source.
func NewConfigMapSource(cfg Config) (source.Source, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, errors.New("empty configmap name")
	}
	return newSource(KindConfigMap, resourceTypeConfigMap, cfg), nil
}

// NewSecretSource creates a Secret-backed source.
func NewSecretSource(cfg Config) (source.Source, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, errors.New("empty secret name")
	}
	return newSource(KindSecret, resourceTypeSecret, cfg), nil
}

func newSource(kind string, resourceType string, cfg Config) *configSource {
	factory := kube.NewClientFactory()
	return &configSource{
		kind:            kind,
		resourceType:    resourceType,
		cfg:             cfg,
		watch:           cfg.Watch,
		clientForConfig: factory.Client,
		closeCh:         make(chan struct{}),
	}
}

func (s *configSource) Kind() string { return s.kind }

func (s *configSource) Name() string { return s.cfg.Name }

func (s *configSource) Read() (source.Data, error) {
	data, parser, err := s.fetch()
	if err != nil {
		return nil, err
	}
	if s.cfg.MergeAllKeys {
		return source.NewMapData(data), nil
	}

	key := s.cfg.Key
	if key == "" {
		key = inferKeyFromData(data)
	}
	value, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("key %q is not a string", key)
	}
	if parser == nil {
		parser = inferParser(key)
	}
	return source.NewBytesData([]byte(str), parser), nil
}

func (s *configSource) Watch() (<-chan source.Data, error) {
	if !s.watch {
		return nil, errors.New("watch disabled for this source")
	}

	client, err := s.clientForConfig(s.cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-s.closeCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	out := make(chan source.Data)
	go func() {
		defer close(out)
		defer cancel()

		var last string
		for {
			ch, err := s.doWatch(ctx, client)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				time.Sleep(time.Second)
				continue
			}

			for event := range ch {
				if event.Type == watch.Deleted {
					return
				}
				if event.Type != watch.Added && event.Type != watch.Modified {
					continue
				}

				data, parser, err := s.fetch()
				if err != nil {
					continue
				}
				payload, content, err := s.payload(data, parser)
				if err != nil {
					continue
				}
				if content == last {
					continue
				}
				last = content
				out <- payload
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	return out, nil
}

func (s *configSource) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
	})
	return nil
}

func (s *configSource) payload(
	data map[string]any,
	parser source.Parser,
) (source.Data, string, error) {
	if s.cfg.MergeAllKeys {
		payload := source.NewMapData(data)
		return payload, string(payload.Bytes()), nil
	}

	key := s.cfg.Key
	if key == "" {
		key = inferKeyFromData(data)
	}
	value, ok := data[key]
	if !ok {
		return nil, "", fmt.Errorf("key %q not found", key)
	}
	str, ok := value.(string)
	if !ok {
		return nil, "", fmt.Errorf("key %q is not a string", key)
	}
	if parser == nil {
		parser = inferParser(key)
	}
	return source.NewBytesData([]byte(str), parser), str, nil
}

func (s *configSource) fetch() (map[string]any, source.Parser, error) {
	client, err := s.clientForConfig(s.cfg.Kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get kube client: %w", err)
	}

	var (
		data   map[string]any
		parser source.Parser
	)
	if s.resourceType == resourceTypeConfigMap {
		cm, err := client.CoreV1().ConfigMaps(s.cfg.Namespace).Get(
			context.Background(),
			s.cfg.Name,
			metav1.GetOptions{},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get configmap: %w", err)
		}
		data = make(map[string]any, len(cm.Data))
		for key, value := range cm.Data {
			data[key] = value
		}
	} else {
		secret, err := client.CoreV1().Secrets(s.cfg.Namespace).Get(
			context.Background(),
			s.cfg.Name,
			metav1.GetOptions{},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get secret: %w", err)
		}
		data = make(map[string]any, len(secret.Data))
		for key, value := range secret.Data {
			data[key] = string(value)
		}
	}

	if s.cfg.Format != nil {
		parser = s.cfg.Format
	} else if s.cfg.Key != "" {
		parser = inferParser(s.cfg.Key)
	}
	return data, parser, nil
}

func (s *configSource) doWatch(
	ctx context.Context,
	client kubernetes.Interface,
) (<-chan watch.Event, error) {
	opts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", s.cfg.Name),
	}
	var (
		w   watch.Interface
		err error
	)
	if s.resourceType == resourceTypeConfigMap {
		w, err = client.CoreV1().ConfigMaps(s.cfg.Namespace).Watch(ctx, opts)
	} else {
		w, err = client.CoreV1().Secrets(s.cfg.Namespace).Watch(ctx, opts)
	}
	if err != nil {
		return nil, err
	}
	return w.ResultChan(), nil
}

func inferParser(key string) source.Parser {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".json":
		return json.Unmarshal
	case ".toml":
		if parser, err := source.ParseParser("toml"); err == nil {
			return parser
		}
	case ".yaml", ".yml":
		if parser, err := source.ParseParser("yaml"); err == nil {
			return parser
		}
	}
	parser, _ := source.ParseParser("yaml")
	return parser
}

func inferKeyFromData(data map[string]any) string {
	for key := range data {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, ".yaml") ||
			strings.Contains(lowerKey, ".yml") ||
			strings.Contains(lowerKey, ".json") ||
			strings.Contains(lowerKey, ".toml") {
			return key
		}
	}
	for key := range data {
		return key
	}
	return "config"
}
