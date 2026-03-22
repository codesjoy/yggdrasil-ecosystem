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

package xds

import (
	"log"
	"slices"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

func (c *adsClient) resendSubscriptions() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, typeURL := range subscriptionTypeURLs() {
		if len(c.resourceNamesLocked(typeURL)) > 0 {
			c.sendSubscriptionRequestLocked(typeURL)
		}
	}
}

// sendSubscriptionRequestLocked assumes c.mu is held.
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
		log.Printf("[xDS] send buffer full, dropping subscription request for %s", typeURL)
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
		case req := <-c.sendCh:
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

	newSub := subscriptions{lds: lds, rds: rds, cds: cds, eds: eds}
	if subscriptionsEqual(c.sub, newSub) {
		return
	}

	c.sub = newSub
	for _, typeURL := range subscriptionTypeURLs() {
		c.sendSubscriptionRequestLocked(typeURL)
	}
}

func (c *adsClient) handleResponse(resp *discoveryv3.DiscoveryResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	events, err := decodeDiscoveryResponse(resp.TypeUrl, resp.Resources)
	if err != nil {
		log.Printf("[xDS] failed to decode response: %v", err)
		c.sendNACK(resp.TypeUrl, resp.VersionInfo, resp.Nonce, err.Error())
		return
	}

	for _, event := range events {
		if c.handle != nil {
			c.handle(event)
		}
	}

	state := c.watchStateLocked(resp.TypeUrl)
	state.version = resp.VersionInfo
	state.nonce = resp.Nonce
	for _, res := range resp.Resources {
		state.resources[getResourceName(res)] = res
	}

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

func getResourceName(msg *anypb.Any) string {
	if msg == nil {
		return ""
	}
	return string(msg.Value)
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

	state := &typeWatchState{resources: make(map[string]*anypb.Any)}
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
