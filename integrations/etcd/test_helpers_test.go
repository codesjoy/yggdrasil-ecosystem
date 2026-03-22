package etcd

import (
	"context"
	"sync"
	"testing"
	"time"

	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeEtcdClient struct {
	getFunc       func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	putFunc       func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error)
	deleteFunc    func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
	grantFunc     func(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	keepAliveFunc func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	watchFunc     func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan
	closeFunc     func() error
}

func (f *fakeEtcdClient) Get(
	ctx context.Context,
	key string,
	opts ...clientv3.OpOption,
) (*clientv3.GetResponse, error) {
	if f.getFunc != nil {
		return f.getFunc(ctx, key, opts...)
	}
	return &clientv3.GetResponse{}, nil
}

func (f *fakeEtcdClient) Put(
	ctx context.Context,
	key, val string,
	opts ...clientv3.OpOption,
) (*clientv3.PutResponse, error) {
	if f.putFunc != nil {
		return f.putFunc(ctx, key, val, opts...)
	}
	return &clientv3.PutResponse{}, nil
}

func (f *fakeEtcdClient) Delete(
	ctx context.Context,
	key string,
	opts ...clientv3.OpOption,
) (*clientv3.DeleteResponse, error) {
	if f.deleteFunc != nil {
		return f.deleteFunc(ctx, key, opts...)
	}
	return &clientv3.DeleteResponse{}, nil
}

func (f *fakeEtcdClient) Grant(
	ctx context.Context,
	ttl int64,
) (*clientv3.LeaseGrantResponse, error) {
	if f.grantFunc != nil {
		return f.grantFunc(ctx, ttl)
	}
	return &clientv3.LeaseGrantResponse{}, nil
}

func (f *fakeEtcdClient) KeepAlive(
	ctx context.Context,
	id clientv3.LeaseID,
) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if f.keepAliveFunc != nil {
		return f.keepAliveFunc(ctx, id)
	}
	ch := make(chan *clientv3.LeaseKeepAliveResponse)
	close(ch)
	return ch, nil
}

func (f *fakeEtcdClient) Watch(
	ctx context.Context,
	key string,
	opts ...clientv3.OpOption,
) clientv3.WatchChan {
	if f.watchFunc != nil {
		return f.watchFunc(ctx, key, opts...)
	}
	ch := make(chan clientv3.WatchResponse)
	close(ch)
	return ch
}

func (f *fakeEtcdClient) Close() error {
	if f.closeFunc != nil {
		return f.closeFunc()
	}
	return nil
}

type testInstance struct {
	namespace string
	name      string
	version   string
	region    string
	zone      string
	campus    string
	metadata  map[string]string
	endpoints []yregistry.Endpoint
}

func (d testInstance) Region() string                  { return d.region }
func (d testInstance) Zone() string                    { return d.zone }
func (d testInstance) Campus() string                  { return d.campus }
func (d testInstance) Namespace() string               { return d.namespace }
func (d testInstance) Name() string                    { return d.name }
func (d testInstance) Version() string                 { return d.version }
func (d testInstance) Metadata() map[string]string     { return d.metadata }
func (d testInstance) Endpoints() []yregistry.Endpoint { return d.endpoints }

type testEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
}

func (d testEndpoint) Scheme() string              { return d.scheme }
func (d testEndpoint) Address() string             { return d.address }
func (d testEndpoint) Metadata() map[string]string { return d.metadata }

type captureWatcher struct {
	mu     sync.Mutex
	states []yresolver.State
	ch     chan yresolver.State
}

func newCaptureWatcher(buffer int) *captureWatcher {
	return &captureWatcher{ch: make(chan yresolver.State, buffer)}
}

func (w *captureWatcher) UpdateState(st yresolver.State) {
	w.mu.Lock()
	w.states = append(w.states, st)
	w.mu.Unlock()
	select {
	case w.ch <- st:
	default:
	}
}

func getResp(rev int64, kvs ...*mvccpb.KeyValue) *clientv3.GetResponse {
	return &clientv3.GetResponse{Header: &pb.ResponseHeader{Revision: rev}, Kvs: kvs}
}

func kv(key, value string) *mvccpb.KeyValue {
	return &mvccpb.KeyValue{Key: []byte(key), Value: []byte(value)}
}

func immediateAfter(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

func mustReceiveState(t *testing.T, ch <-chan yresolver.State) yresolver.State {
	t.Helper()
	select {
	case st := <-ch:
		return st
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for state")
		return nil
	}
}

func mustNotReceiveState(t *testing.T, ch <-chan yresolver.State) {
	t.Helper()
	select {
	case st := <-ch:
		t.Fatalf("unexpected state: %#v", st)
	case <-time.After(150 * time.Millisecond):
	}
}

func boolPtrValue(v bool) *bool {
	return &v
}
