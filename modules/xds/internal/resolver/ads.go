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

package resolver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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
	version string
	nonce   string
}

type adsClient struct {
	cfg        Config
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	conn       *grpc.ClientConn
	stream     discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient
	node       *corev3.Node
	sub        subscriptions
	typeState  map[string]*typeWatchState
	handle     func(xdsresource.DiscoveryEvent)
	sendCh     chan *discoveryv3.DiscoveryRequest
	retries    int
	maxRetries int
	closeOnce  sync.Once
}

func newADSClient(cfg Config, handle func(xdsresource.DiscoveryEvent)) (*adsClient, error) {
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
		maxRetries: maxADSRetries(cfg),
	}, nil
}

func newADSNode(cfg Config) (*corev3.Node, error) {
	metadataMap := make(map[string]any, len(cfg.Node.Metadata))
	for key, value := range cfg.Node.Metadata {
		metadataMap[key] = value
	}

	metadata, err := structpb.NewStruct(metadataMap)
	if err != nil {
		return nil, err
	}

	node := &corev3.Node{
		Id:       cfg.Node.ID,
		Cluster:  cfg.Node.Cluster,
		Metadata: metadata,
	}
	if cfg.Node.Locality != nil {
		node.Locality = &corev3.Locality{
			Region:  cfg.Node.Locality.Region,
			Zone:    cfg.Node.Locality.Zone,
			SubZone: cfg.Node.Locality.SubZone,
		}
	}

	return node, nil
}

func maxADSRetries(cfg Config) int {
	if cfg.MaxRetries > 0 {
		return cfg.MaxRetries
	}
	return defaultADSMaxRetries
}

func (c *adsClient) Start() error {
	if c.cfg.Server.Address == "" {
		return fmt.Errorf("xds server address is empty")
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
			log.Printf("[xds] connection failed: %v, retrying in %v", err, backoff)
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

	//nolint:staticcheck // SA1019: DialContext is deprecated but still supported in gRPC 1.x.
	conn, err := grpc.DialContext(ctx, c.cfg.Server.Address, opts...)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck

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

	//nolint:gosec // G402: MinVersion is intentionally not set to allow configuration flexibility.
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.cfg.Server.TLS.CertFile != "" && c.cfg.Server.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.cfg.Server.TLS.CertFile, c.cfg.Server.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS cert pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if c.cfg.Server.TLS.CAFile != "" {
		caCert, err := os.ReadFile(c.cfg.Server.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
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

func (c *adsClient) resendSubscriptions() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, typeURL := range subscriptionTypeURLs() {
		if len(c.resourceNamesLocked(typeURL)) > 0 {
			c.sendSubscriptionRequestLocked(typeURL)
		}
	}
}

func (c *adsClient) sendSubscriptionRequestLocked(typeURL string) {
	req := &discoveryv3.DiscoveryRequest{
		Node:          c.node,
		TypeUrl:       typeURL,
		ResourceNames: c.resourceNamesLocked(typeURL),
	}
	c.applyWatchStateLocked(req, c.watchStateLocked(typeURL))

	select {
	case c.sendCh <- req:
	default:
		log.Printf("[xds] send buffer full, dropping subscription request for %s", typeURL)
	}
}

func (c *adsClient) sendLoop(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient,
) error {
	for {
		select {
		case <-c.ctx.Done():
			return nil
		case <-stream.Context().Done():
			return stream.Context().Err()
		case req, ok := <-c.sendCh:
			if !ok {
				return nil
			}
			if err := stream.Send(req); err != nil {
				return err
			}
		}
	}
}

func (c *adsClient) watchResources(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesClient,
) error {
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		c.handleResponse(resp)
	}
}

func (c *adsClient) UpdateSubscriptions(lds, rds, cds, eds []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	slices.Sort(lds)
	slices.Sort(rds)
	slices.Sort(cds)
	slices.Sort(eds)

	next := subscriptions{lds: lds, rds: rds, cds: cds, eds: eds}
	if subscriptionsEqual(c.sub, next) {
		return
	}

	c.sub = next
	for _, typeURL := range subscriptionTypeURLs() {
		c.sendSubscriptionRequestLocked(typeURL)
	}
}

func (c *adsClient) handleResponse(resp *discoveryv3.DiscoveryResponse) {
	events, err := xdsresource.DecodeDiscoveryResponse(resp.TypeUrl, resp.Resources)
	if err != nil {
		log.Printf("[xds] failed to decode response: %v", err)
		c.sendNACK(resp.TypeUrl, resp.VersionInfo, resp.Nonce, err.Error())
		return
	}

	for _, event := range events {
		if c.handle != nil {
			c.handle(event)
		}
	}

	c.mu.Lock()
	state := c.watchStateLocked(resp.TypeUrl)
	state.version = resp.VersionInfo
	state.nonce = resp.Nonce
	c.mu.Unlock()

	c.sendACK(resp.TypeUrl, resp.VersionInfo, resp.Nonce)
}

func (c *adsClient) sendACK(typeURL, version, nonce string) {
	req := &discoveryv3.DiscoveryRequest{
		Node:          c.node,
		TypeUrl:       typeURL,
		VersionInfo:   version,
		ResponseNonce: nonce,
	}

	select {
	case c.sendCh <- req:
	default:
	}
}

func (c *adsClient) sendNACK(typeURL, version, nonce, errMsg string) {
	req := &discoveryv3.DiscoveryRequest{
		Node:          c.node,
		TypeUrl:       typeURL,
		VersionInfo:   version,
		ResponseNonce: nonce,
		ErrorDetail: &status.Status{
			Message: errMsg,
		},
	}

	select {
	case c.sendCh <- req:
	default:
	}
}

func subscriptionsEqual(a, b subscriptions) bool {
	return slices.Equal(a.lds, b.lds) &&
		slices.Equal(a.rds, b.rds) &&
		slices.Equal(a.cds, b.cds) &&
		slices.Equal(a.eds, b.eds)
}

func subscriptionTypeURLs() []string {
	return []string{
		resource.ListenerType,
		resource.RouteType,
		resource.ClusterType,
		resource.EndpointType,
	}
}

func (c *adsClient) resourceNamesLocked(typeURL string) []string {
	switch typeURL {
	case resource.ListenerType:
		return c.sub.lds
	case resource.RouteType:
		return c.sub.rds
	case resource.ClusterType:
		return c.sub.cds
	case resource.EndpointType:
		return c.sub.eds
	default:
		return nil
	}
}

func (c *adsClient) watchStateLocked(typeURL string) *typeWatchState {
	if state := c.typeState[typeURL]; state != nil {
		return state
	}

	state := &typeWatchState{}
	c.typeState[typeURL] = state
	return state
}

func (c *adsClient) applyWatchStateLocked(
	req *discoveryv3.DiscoveryRequest,
	state *typeWatchState,
) {
	req.VersionInfo = state.version
	req.ResponseNonce = state.nonce
}

func (c *adsClient) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
		if c.stream != nil {
			_ = c.stream.CloseSend()
		}
		if c.conn != nil {
			_ = c.conn.Close()
		}
		close(c.sendCh)
	})
}
