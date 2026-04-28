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
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

const (
	// Kind is the declarative config source kind for etcd-backed config layers.
	Kind = "etcd"
	// ModeBlob loads one full document from a single etcd key.
	ModeBlob = "blob"
	// ModeKV loads one structured map from a key prefix.
	ModeKV = "kv"
)

// Config configures one etcd-backed config source.
type Config struct {
	Client string        `mapstructure:"client"`
	Key    string        `mapstructure:"key"`
	Prefix string        `mapstructure:"prefix"`
	Mode   string        `mapstructure:"mode"`
	Watch  *bool         `mapstructure:"watch"`
	Format source.Parser `mapstructure:"format"`
	Name   string        `mapstructure:"name"`
}

// NewConfigSource creates one etcd-backed config source.
func NewConfigSource(cfg Config) (source.Source, error) {
	mode := cfg.Mode
	if mode == "" {
		switch {
		case strings.TrimSpace(cfg.Key) != "":
			mode = ModeBlob
		case strings.TrimSpace(cfg.Prefix) != "":
			mode = ModeKV
		default:
			mode = ModeBlob
		}
	}
	cfg.Mode = mode
	if cfg.Format == nil {
		cfg.Format = yaml.Unmarshal
	}

	if strings.TrimSpace(cfg.Key) == "" && strings.TrimSpace(cfg.Prefix) == "" {
		return nil, errors.New("empty etcd config key/prefix")
	}
	if strings.TrimSpace(cfg.Key) != "" && strings.TrimSpace(cfg.Prefix) != "" {
		return nil, errors.New("both etcd config key and prefix are set")
	}

	clientCfg := internalclient.LoadConfig(cfg.Client)
	cli, err := internalclient.New(clientCfg)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		if strings.TrimSpace(cfg.Key) != "" {
			name = cfg.Key
		} else {
			name = cfg.Prefix
		}
	}

	return &configSource{
		name:        name,
		cfg:         cfg,
		cli:         cli,
		client:      internalclient.Wrap(cli),
		watch:       cfg.Watch == nil || *cfg.Watch,
		dialTimeout: clientCfg.DialTimeout,
		closeCh:     make(chan struct{}),
	}, nil
}

type configSource struct {
	name        string
	cfg         Config
	cli         *clientv3.Client
	client      internalclient.Client
	watch       bool
	dialTimeout time.Duration

	closeOnce sync.Once
	closeCh   chan struct{}
}

func (s *configSource) Kind() string { return Kind }

func (s *configSource) Name() string { return s.name }

func (s *configSource) Read() (source.Data, error) {
	switch s.cfg.Mode {
	case ModeBlob:
		return s.readBlob()
	case ModeKV:
		return s.readKV()
	default:
		return nil, errors.New("unknown etcd config source mode")
	}
}

func (s *configSource) Watch() (<-chan source.Data, error) {
	if !s.watch {
		return nil, errors.New("etcd config source is not changeable")
	}

	out := make(chan source.Data)
	go func() {
		defer close(out)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			<-s.closeCh
			cancel()
		}()

		ch := s.client.Watch(ctx, s.watchKey(), s.watchOptions()...)
		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-ch:
				if !ok || resp.Canceled {
					return
				}
				data, err := s.Read()
				if err != nil {
					continue
				}
				out <- data
			}
		}
	}()
	return out, nil
}

func (s *configSource) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
		if s.client != nil {
			_ = s.client.Close()
		}
	})
	return nil
}

func (s *configSource) watchKey() string {
	if strings.TrimSpace(s.cfg.Key) != "" {
		return s.cfg.Key
	}
	return s.cfg.Prefix
}

func (s *configSource) watchOptions() []clientv3.OpOption {
	if s.cfg.Mode == ModeKV {
		return []clientv3.OpOption{clientv3.WithPrefix()}
	}
	return nil
}

func (s *configSource) readBlob() (source.Data, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.dialTimeout)
	defer cancel()
	resp, err := s.client.Get(ctx, s.cfg.Key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return source.NewBytesData(nil, s.cfg.Format), nil
	}
	return source.NewBytesData(resp.Kvs[0].Value, s.cfg.Format), nil
}

func (s *configSource) readKV() (source.Data, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.dialTimeout)
	defer cancel()
	resp, err := s.client.Get(ctx, s.cfg.Prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	out := map[string]any{}
	for _, item := range resp.Kvs {
		rel := strings.TrimPrefix(string(item.Key), s.cfg.Prefix)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			continue
		}
		parts := splitConfigPath(strings.ReplaceAll(rel, "/", "."), ".")
		setNested(out, parts, parseScalarOrDoc(item.Value, s.cfg.Format))
	}
	return source.NewMapData(out), nil
}

func parseScalarOrDoc(data []byte, parser source.Parser) any {
	if parser == nil {
		return string(data)
	}
	var out any
	if err := parser(data, &out); err == nil {
		return out
	}
	return string(data)
}

var keyRegx = regexp.MustCompile(`{([\w.-]+)}`)

func splitConfigPath(key string, delimiter string) []string {
	matches := make([]string, 0)
	key = keyRegx.ReplaceAllStringFunc(key, func(item string) string {
		matches = append(matches, item[1:len(item)-1])
		return "{}"
	})
	parts := strings.Split(key, delimiter)
	matchIndex := 0
	for i, item := range parts {
		if item == "{}" {
			parts[i] = matches[matchIndex]
			matchIndex++
		}
	}
	return parts
}

func setNested(dst map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}
	head, tail := path[0], path[1:]
	if len(tail) == 0 {
		dst[head] = value
		return
	}
	next, ok := dst[head]
	if !ok {
		child := map[string]any{}
		dst[head] = child
		next = child
	}
	nextMap, ok := next.(map[string]any)
	if !ok {
		dst[head] = value
		return
	}
	setNested(nextMap, tail, value)
}
