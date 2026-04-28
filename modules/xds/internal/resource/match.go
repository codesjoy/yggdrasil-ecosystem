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
	"net"
	"strings"
)

// MatchRoute finds the matching route action for a given path and headers.
func MatchRoute(vhosts []*VirtualHost, path string, headers map[string]string) *RouteAction {
	if len(vhosts) == 0 {
		return nil
	}

	vhost := selectVirtualHost(vhosts, requestHost(headers))
	if vhost == nil {
		vhost = vhosts[0]
	}

	for _, route := range vhost.Routes {
		if route.Match.Matches(path, headers) {
			return route.Action
		}
	}

	return nil
}

// Matches checks if the route match rules apply to the request.
func (m *RouteMatch) Matches(path string, headers map[string]string) bool {
	if m == nil {
		return true
	}

	return m.matchPath(path) && m.matchHeaders(headers)
}

func (m *RouteMatch) matchPath(path string) bool {
	switch {
	case m.Path != "":
		return path == m.Path
	case m.Prefix != "":
		return strings.HasPrefix(path, m.Prefix)
	case m.Suffix != "":
		return strings.HasSuffix(path, m.Suffix)
	case m.Contains != "":
		return strings.Contains(path, m.Contains)
	case m.Regex != nil:
		return m.Regex.MatchString(path)
	default:
		return true
	}
}

func (m *RouteMatch) matchHeaders(headers map[string]string) bool {
	for _, header := range m.Headers {
		if !header.matches(headers) {
			return false
		}
	}
	return true
}

func (h *HeaderMatcher) matches(headers map[string]string) bool {
	value, ok := headers[h.Name]
	if !ok {
		return false
	}
	if h.Present {
		return true
	}
	if h.ExactMatch != "" && value != h.ExactMatch {
		return false
	}
	if h.PrefixMatch != "" && !strings.HasPrefix(value, h.PrefixMatch) {
		return false
	}
	if h.SuffixMatch != "" && !strings.HasSuffix(value, h.SuffixMatch) {
		return false
	}
	if h.RegexMatch != nil && !h.RegexMatch.MatchString(value) {
		return false
	}
	return true
}

func requestHost(headers map[string]string) string {
	if host := normalizeHost(headers[":authority"]); host != "" {
		return host
	}
	return normalizeHost(headers["host"])
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.Count(host, ":") == 1 {
		if cutHost, _, ok := strings.Cut(host, ":"); ok {
			host = cutHost
		}
	}

	return strings.Trim(host, "[]")
}

func selectVirtualHost(vhosts []*VirtualHost, host string) *VirtualHost {
	bestScore := 0
	var selected *VirtualHost

	for _, vhost := range vhosts {
		score := virtualHostScore(vhost, host)
		if score > bestScore {
			bestScore = score
			selected = vhost
		}
	}

	return selected
}

func virtualHostScore(vhost *VirtualHost, host string) int {
	if vhost == nil {
		return 0
	}
	if len(vhost.Domains) == 0 {
		if host == "" {
			return 1
		}
		return 0
	}

	best := 0
	for _, domain := range vhost.Domains {
		score := domainMatchScore(domain, host)
		if score > best {
			best = score
		}
	}
	return best
}

func domainMatchScore(domain, host string) int {
	domain = normalizeHost(domain)
	switch {
	case domain == "":
		return 0
	case domain == "*":
		return 1
	case host == "":
		return 0
	case domain == host:
		return 4
	case strings.HasPrefix(domain, "*."):
		suffix := domain[1:]
		if strings.HasSuffix(host, suffix) && host != suffix[1:] {
			return 3
		}
	case strings.HasSuffix(domain, ".*"):
		prefix := strings.TrimSuffix(domain, "*")
		if strings.HasPrefix(host, prefix) {
			return 2
		}
	}

	return 0
}
