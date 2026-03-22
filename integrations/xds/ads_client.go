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

// Package xds provides xDS (Envoy discovery service) integration for service discovery and configuration.
// It implements the ADS (Aggregated Discovery Service) protocol for dynamic configuration updates.
package xds

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultADSMaxRetries = 10
	adsSendBufferSize    = 100
)

type subscriptions struct {
	lds []string
	rds []string
	cds []string
	eds []string
}

type typeWatchState struct {
	version   string
	nonce     string
	resources map[string]*anypb.Any
}

type adsClient struct {
	cfg        ResolverConfig
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	conn       *grpc.ClientConn
	stream     discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient
	node       *corev3.Node
	sub        subscriptions
	typeState  map[string]*typeWatchState
	handle     func(discoveryEvent)
	sendCh     chan *discoveryv3.DiscoveryRequest
	retries    int
	maxRetries int
}

func newADSClient(cfg ResolverConfig, handle func(discoveryEvent)) (*adsClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	node, err := newADSNode(cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	return &adsClient{
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
		node:       node,
		sub:        subscriptions{},
		typeState:  make(map[string]*typeWatchState),
		handle:     handle,
		sendCh:     make(chan *discoveryv3.DiscoveryRequest, adsSendBufferSize),
		retries:    0,
		maxRetries: maxADSRetries(cfg),
	}, nil
}

func newADSNode(cfg ResolverConfig) (*corev3.Node, error) {
	metadataMap := make(map[string]any, len(cfg.Node.Metadata))
	for key, value := range cfg.Node.Metadata {
		metadataMap[key] = value
	}

	metadata, err := structpb.NewStruct(metadataMap)
	if err != nil {
		return nil, err
	}

	return &corev3.Node{
		Id:       cfg.Node.ID,
		Cluster:  cfg.Node.Cluster,
		Metadata: metadata,
	}, nil
}

func maxADSRetries(cfg ResolverConfig) int {
	if cfg.MaxRetries > 0 {
		return cfg.MaxRetries
	}
	return defaultADSMaxRetries
}

func (c *adsClient) Start() error {
	if c.cfg.Server.Address == "" {
		return fmt.Errorf("xDS server address is empty")
	}
	go c.run()
	return nil
}

func (c *adsClient) run() {
	backoff := time.Second
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if err := c.connect(); err != nil {
			log.Printf("[xDS] connection failed: %v, retrying in %v", err, backoff)
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(backoff):
				if c.shouldReconnect() {
					c.incrementRetry()
					backoff = time.Duration(c.retries) * time.Second
				} else {
					return
				}
			}
			continue
		}

		c.resetRetries()
		backoff = time.Second
	}
}

func (c *adsClient) connect() error {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*10),
			grpc.MaxCallSendMsgSize(1024*1024*10),
		),
	}

	transportCredentials, err := c.transportCredentials()
	if err != nil {
		return err
	}
	opts = append(opts, grpc.WithTransportCredentials(transportCredentials))

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Server.Timeout)
	defer cancel()

	//nolint:staticcheck // SA1019: DialContext is deprecated but still supported in gRPC 1.x
	conn, err := grpc.DialContext(ctx, c.cfg.Server.Address, opts...)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint

	client := discoveryv3.NewAggregatedDiscoveryServiceClient(conn)
	stream, err := client.StreamAggregatedResources(c.ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.stream = stream
	c.mu.Unlock()

	c.resendSubscriptions()

	errCh := make(chan error, 2)
	go func() { errCh <- c.sendLoop(stream) }()
	go func() { errCh <- c.watchResources(stream) }()

	select {
	case <-c.ctx.Done():
		return nil
	case err := <-errCh:
		_ = stream.CloseSend()
		return err
	}
}

func (c *adsClient) transportCredentials() (credentials.TransportCredentials, error) {
	if !c.cfg.Server.TLS.Enable {
		return insecure.NewCredentials(), nil
	}

	//nolint:gosec // G402: MinVersion is intentionally not set to allow configuration flexibility
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.cfg.Server.TLS.CertFile != "" && c.cfg.Server.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.cfg.Server.TLS.CertFile, c.cfg.Server.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS cert pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if c.cfg.Server.TLS.CAFile != "" {
		caCert, err := os.ReadFile(c.cfg.Server.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	return credentials.NewTLS(tlsConfig), nil
}

func (c *adsClient) shouldReconnect() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.retries < c.maxRetries
}

func (c *adsClient) incrementRetry() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.retries++
}

func (c *adsClient) resetRetries() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.retries = 0
}

func (c *adsClient) Close() {
	c.cancel()
	if c.stream != nil {
		_ = c.stream.CloseSend()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	close(c.sendCh)
}
