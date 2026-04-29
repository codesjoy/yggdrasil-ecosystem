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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
)

type fakeADSStream struct {
	ctx       context.Context
	sent      []*discoveryv3.DiscoveryRequest
	responses []*discoveryv3.DiscoveryResponse
	recvErr   error
}

func (f *fakeADSStream) Send(req *discoveryv3.DiscoveryRequest) error {
	f.sent = append(f.sent, req)
	return nil
}

func (f *fakeADSStream) Recv() (*discoveryv3.DiscoveryResponse, error) {
	if len(f.responses) == 0 {
		if f.recvErr != nil {
			return nil, f.recvErr
		}
		return nil, io.EOF
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeADSStream) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (f *fakeADSStream) Trailer() metadata.MD         { return metadata.MD{} }
func (f *fakeADSStream) CloseSend() error             { return nil }
func (f *fakeADSStream) Context() context.Context {
	if f.ctx != nil {
		return f.ctx
	}
	return context.Background()
}
func (f *fakeADSStream) SendMsg(any) error { return nil }
func (f *fakeADSStream) RecvMsg(any) error { return nil }

func TestADSClientHelpers(t *testing.T) {
	client, err := newADSClient(Config{
		MaxRetries: 2,
		Node: NodeConfig{
			ID:       "node-a",
			Cluster:  "cluster-a",
			Metadata: map[string]string{"env": "test"},
			Locality: &Locality{Region: "cn", Zone: "hz"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	if client.node.Locality == nil || client.node.Locality.Region != "cn" {
		t.Fatalf("node locality not propagated: %#v", client.node.Locality)
	}
	if err := client.Start(); err == nil {
		t.Fatal("Start() expected empty address error")
	}

	client.sub = subscriptions{
		lds: []string{"listener-a"},
		rds: []string{"route-a"},
		cds: []string{"cluster-a"},
		eds: []string{"cluster-a"},
	}
	client.resendSubscriptions()
	if len(client.sendCh) != 4 {
		t.Fatalf("resendSubscriptions() queued %d requests, want 4", len(client.sendCh))
	}
	client.UpdateSubscriptions([]string{"b", "a"}, []string{"r"}, []string{"c"}, []string{"e"})
	if !subscriptionsEqual(client.sub, subscriptions{
		lds: []string{"a", "b"},
		rds: []string{"r"},
		cds: []string{"c"},
		eds: []string{"e"},
	}) {
		t.Fatalf("unexpected subscriptions: %#v", client.sub)
	}
	if !client.shouldReconnect() {
		t.Fatal("shouldReconnect() = false, want true")
	}
	client.incrementRetry()
	client.incrementRetry()
	if client.shouldReconnect() {
		t.Fatal("shouldReconnect() = true, want false after max retries")
	}
	client.resetRetries()
	if !client.shouldReconnect() {
		t.Fatal("resetRetries() did not reset retry state")
	}
	client.Close()
}

func TestADSResponseHandling(t *testing.T) {
	t.Run("send loop", func(t *testing.T) {
		client, err := newADSClient(Config{
			Node: NodeConfig{ID: "node-a", Cluster: "cluster-a"},
		}, nil)
		if err != nil {
			t.Fatalf("newADSClient() error = %v", err)
		}
		defer client.Close()

		client.ctx, client.cancel = context.WithCancel(context.Background())

		stream := &fakeADSStream{ctx: context.Background()}
		client.sendCh <- &discoveryv3.DiscoveryRequest{TypeUrl: resource.ListenerType}
		client.cancel()
		if err := client.sendLoop(stream); err != nil {
			t.Fatalf("sendLoop() error = %v", err)
		}
	})

	t.Run("response handling", func(t *testing.T) {
		client, err := newADSClient(Config{
			Node: NodeConfig{ID: "node-a", Cluster: "cluster-a"},
		}, nil)
		if err != nil {
			t.Fatalf("newADSClient() error = %v", err)
		}
		defer client.Close()

		invalidResp := &discoveryv3.DiscoveryResponse{
			TypeUrl:     resource.ListenerType,
			VersionInfo: "v1",
			Nonce:       "nonce-1",
			Resources:   []*anypb.Any{{TypeUrl: "bad", Value: []byte("bad")}},
		}
		client.handleResponse(invalidResp)
		select {
		case req := <-client.sendCh:
			if req.ErrorDetail == nil {
				t.Fatal("handleResponse() expected NACK request")
			}
		default:
			t.Fatal("handleResponse() did not enqueue NACK")
		}

		validRoute, _ := anypb.New(&routeType.RouteConfiguration{Name: "route-a"})
		client.handleResponse(&discoveryv3.DiscoveryResponse{
			TypeUrl:     resource.RouteType,
			VersionInfo: "v2",
			Nonce:       "nonce-2",
			Resources:   []*anypb.Any{validRoute},
		})
		select {
		case req := <-client.sendCh:
			if req.ErrorDetail != nil {
				t.Fatal("expected ACK without error detail")
			}
		default:
			t.Fatal("handleResponse() did not enqueue ACK")
		}

		watchStream := &fakeADSStream{
			responses: []*discoveryv3.DiscoveryResponse{{
				TypeUrl:   resource.ClusterType,
				Resources: []*anypb.Any{},
			}},
			recvErr: io.EOF,
		}
		if err := client.watchResources(watchStream); err == nil {
			t.Fatal("watchResources() expected EOF to bubble up")
		}
	})
}

func TestADSClientTransportCredentialsAndConnect(t *testing.T) {
	client, err := newADSClient(DefaultResolverConfig(), nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	defer client.Close()

	creds, err := client.transportCredentials()
	if err != nil {
		t.Fatalf("transportCredentials() error = %v", err)
	}
	if creds == nil {
		t.Fatal("transportCredentials() returned nil credentials")
	}

	dir := t.TempDir()
	certFile, keyFile, caFile := writeTLSFiles(t, dir)

	client.cfg.Server.TLS = TLSConfig{
		Enable:   true,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
	}
	creds, err = client.transportCredentials()
	if err != nil {
		t.Fatalf("transportCredentials(TLS) error = %v", err)
	}
	if creds == nil {
		t.Fatal("transportCredentials(TLS) returned nil credentials")
	}

	client.cfg.Server.TLS = TLSConfig{
		Enable:   true,
		CertFile: certFile,
		KeyFile:  filepath.Join(dir, "missing.key"),
	}
	if _, err := client.transportCredentials(); err == nil {
		t.Fatal("transportCredentials() expected missing key file error")
	}

	client.cfg.Server.TLS = TLSConfig{
		Enable: true,
		CAFile: filepath.Join(dir, "missing-ca.pem"),
	}
	if _, err := client.transportCredentials(); err == nil {
		t.Fatal("transportCredentials() expected missing CA file error")
	}

	client.cfg.Server.Address = "127.0.0.1:1"
	if err := client.connect(); err == nil {
		t.Fatal("connect() expected transport credential error")
	}
}

func TestADSClientStartAndInternalHelpers(t *testing.T) {
	client, err := newADSClient(Config{
		Server: ServerConfig{
			Address: "127.0.0.1:65535",
			Timeout: time.Millisecond,
		},
		Node: NodeConfig{ID: "node-a", Cluster: "cluster-a"},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	if err := client.Start(); err != nil {
		t.Fatalf("Start() error = %v, want nil for non-empty address", err)
	}
	client.Close()

	client, err = newADSClient(Config{
		Node: NodeConfig{
			ID:       "node-a",
			Cluster:  "cluster-a",
			Metadata: map[string]string{"env": "test"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	defer client.Close()

	if names := client.resourceNamesLocked("unknown"); names != nil {
		t.Fatalf("resourceNamesLocked(unknown) = %#v, want nil", names)
	}

	state := client.watchStateLocked(resource.ListenerType)
	state.version = "v1"
	state.nonce = "nonce-1"

	req := &discoveryv3.DiscoveryRequest{}
	client.applyWatchStateLocked(req, state)
	if req.VersionInfo != "v1" || req.ResponseNonce != "nonce-1" {
		t.Fatalf("applyWatchStateLocked() = %#v", req)
	}
	if client.watchStateLocked(resource.ListenerType) != state {
		t.Fatal("watchStateLocked() did not reuse the existing watch state")
	}

	client.sub = subscriptions{lds: []string{"listener-a"}}
	for i := 0; i < cap(client.sendCh); i++ {
		client.sendCh <- &discoveryv3.DiscoveryRequest{}
	}
	client.sendSubscriptionRequestLocked(resource.ListenerType)
	if len(client.sendCh) != cap(client.sendCh) {
		t.Fatalf(
			"sendSubscriptionRequestLocked() len = %d, want full buffer %d",
			len(client.sendCh),
			cap(client.sendCh),
		)
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.sendLoop(&fakeADSStream{ctx: streamCtx}); !errors.Is(err, context.Canceled) {
		t.Fatalf("sendLoop() error = %v, want context.Canceled", err)
	}

	client.stream = &fakeADSStream{}
	client.Close()
	client.Close()
}

func writeTLSFiles(t *testing.T, dir string) (string, string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "xds-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	certOut := pemEncode("CERTIFICATE", der)
	keyOut := pemEncode("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))

	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	caFile := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(certFile, certOut, 0o600); err != nil {
		t.Fatalf("WriteFile(cert.pem) error = %v", err)
	}
	if err := os.WriteFile(keyFile, keyOut, 0o600); err != nil {
		t.Fatalf("WriteFile(key.pem) error = %v", err)
	}
	if err := os.WriteFile(caFile, certOut, 0o600); err != nil {
		t.Fatalf("WriteFile(ca.pem) error = %v", err)
	}
	return certFile, keyFile, caFile
}

func pemEncode(blockType string, data []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data})
}
