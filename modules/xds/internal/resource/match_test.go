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

package resource

import (
	"regexp"
	"testing"
)

func TestMatchRouteSelectsVirtualHostByDomain(t *testing.T) {
	vhosts := []*VirtualHost{
		{
			Name:    "wildcard",
			Domains: []string{"*"},
			Routes: []*Route{{
				Match:  &RouteMatch{Prefix: "/"},
				Action: &RouteAction{Cluster: "default"},
			}},
		},
		{
			Name:    "exact",
			Domains: []string{"api.example.com"},
			Routes: []*Route{{
				Match:  &RouteMatch{Prefix: "/"},
				Action: &RouteAction{Cluster: "exact"},
			}},
		},
	}

	action := MatchRoute(vhosts, "/users", map[string]string{":authority": "api.example.com:443"})
	if action == nil || action.Cluster != "exact" {
		t.Fatalf("MatchRoute() = %#v, want exact", action)
	}
}

func TestMatchRouteUsesPathAndHeaders(t *testing.T) {
	vhosts := []*VirtualHost{{
		Name:    "default",
		Domains: []string{"*"},
		Routes: []*Route{
			{
				Match: &RouteMatch{
					Prefix: "/api",
					Headers: []*HeaderMatcher{
						{Name: "x-env", ExactMatch: "prod"},
						{Name: "x-user", RegexMatch: regexp.MustCompile("^user-")},
					},
				},
				Action: &RouteAction{Cluster: "cluster-a"},
			},
			{
				Match:  &RouteMatch{Prefix: "/"},
				Action: &RouteAction{Cluster: "default"},
			},
		},
	}}

	action := MatchRoute(vhosts, "/api/v1", map[string]string{
		"x-env":  "prod",
		"x-user": "user-1",
	})
	if action == nil || action.Cluster != "cluster-a" {
		t.Fatalf("MatchRoute() = %#v, want cluster-a", action)
	}

	action = MatchRoute(vhosts, "/api/v1", map[string]string{"x-env": "stage"})
	if action == nil || action.Cluster != "default" {
		t.Fatalf("fallback MatchRoute() = %#v, want default", action)
	}
}
