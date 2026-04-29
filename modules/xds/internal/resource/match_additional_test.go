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

func TestRouteMatchPathStrategies(t *testing.T) {
	tests := []struct {
		name  string
		match *RouteMatch
		path  string
		want  bool
	}{
		{name: "nil match", match: nil, path: "/any", want: true},
		{name: "exact", match: &RouteMatch{Path: "/exact"}, path: "/exact", want: true},
		{name: "prefix", match: &RouteMatch{Prefix: "/api"}, path: "/api/v1", want: true},
		{name: "suffix", match: &RouteMatch{Suffix: ".json"}, path: "/users.json", want: true},
		{
			name:  "contains",
			match: &RouteMatch{Contains: "beta"},
			path:  "/feature/beta/list",
			want:  true,
		},
		{
			name:  "regex",
			match: &RouteMatch{Regex: regexp.MustCompile("^/v[0-9]+/")},
			path:  "/v2/users",
			want:  true,
		},
		{name: "no match", match: &RouteMatch{Path: "/exact"}, path: "/other", want: false},
	}

	for _, tt := range tests {
		if got := tt.match.Matches(tt.path, map[string]string{}); got != tt.want {
			t.Fatalf("%s: Matches() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHeaderMatchersAndVirtualHostSelection(t *testing.T) {
	headers := map[string]string{
		"x-present": "yes",
		"x-prefix":  "prefix-value",
		"x-suffix":  "value-suffix",
		"x-regex":   "user-42",
	}

	if !(&HeaderMatcher{Name: "x-present", Present: true}).matches(headers) {
		t.Fatal("Present matcher did not match")
	}
	if (&HeaderMatcher{Name: "x-missing", Present: true}).matches(headers) {
		t.Fatal("missing header unexpectedly matched")
	}
	if !(&HeaderMatcher{Name: "x-prefix", PrefixMatch: "prefix"}).matches(headers) {
		t.Fatal("Prefix matcher did not match")
	}
	if !(&HeaderMatcher{Name: "x-suffix", SuffixMatch: "suffix"}).matches(headers) {
		t.Fatal("Suffix matcher did not match")
	}
	if !(&HeaderMatcher{Name: "x-regex", RegexMatch: regexp.MustCompile("^user-")}).matches(
		headers,
	) {
		t.Fatal("Regex matcher did not match")
	}

	vhosts := []*VirtualHost{
		{
			Name:    "first",
			Domains: []string{"api.internal"},
			Routes: []*Route{
				{Match: &RouteMatch{Prefix: "/"}, Action: &RouteAction{Cluster: "first"}},
			},
		},
		{
			Name:    "second",
			Domains: []string{"*.example.com"},
			Routes: []*Route{
				{Match: &RouteMatch{Prefix: "/"}, Action: &RouteAction{Cluster: "second"}},
			},
		},
	}

	action := MatchRoute(vhosts, "/users", map[string]string{"host": "unknown.host"})
	if action == nil || action.Cluster != "first" {
		t.Fatalf("MatchRoute(fallback first vhost) = %#v, want first", action)
	}

	selected := selectVirtualHost([]*VirtualHost{
		{
			Name: "empty",
			Routes: []*Route{
				{Match: &RouteMatch{Prefix: "/"}, Action: &RouteAction{Cluster: "empty"}},
			},
		},
	}, "")
	if selected == nil || selected.Name != "empty" {
		t.Fatalf("selectVirtualHost(empty domains) = %#v", selected)
	}
}

func TestHostMatchingHelpers(t *testing.T) {
	if got := normalizeHost(" API.EXAMPLE.COM:443 "); got != "api.example.com" {
		t.Fatalf("normalizeHost(host:port) = %q, want api.example.com", got)
	}
	if got := normalizeHost("[2001:db8::1]:8443"); got != "2001:db8::1" {
		t.Fatalf("normalizeHost(ipv6) = %q, want 2001:db8::1", got)
	}

	tests := []struct {
		domain string
		host   string
		want   int
	}{
		{domain: "", host: "api.example.com", want: 0},
		{domain: "*", host: "api.example.com", want: 1},
		{domain: "api.example.com", host: "api.example.com", want: 4},
		{domain: "*.example.com", host: "api.example.com", want: 3},
		{domain: "api.*", host: "api.example.com", want: 2},
		{domain: "api.example.com", host: "", want: 0},
	}

	for _, tt := range tests {
		if got := domainMatchScore(tt.domain, tt.host); got != tt.want {
			t.Fatalf("domainMatchScore(%q, %q) = %d, want %d", tt.domain, tt.host, got, tt.want)
		}
	}

	vhost := &VirtualHost{Domains: []string{"*.example.com", "api.*"}}
	if got := virtualHostScore(vhost, "api.example.com"); got != 3 {
		t.Fatalf("virtualHostScore() = %d, want 3", got)
	}
}
