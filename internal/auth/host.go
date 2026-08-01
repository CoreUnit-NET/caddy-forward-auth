package auth

import (
	"strings"

	"github.com/NobleMajo/intern-auth-gateway/internal/config"
)

// HostMatches reports whether host matches the SERVICE_* host glob.
// Matching is case-insensitive. '*' matches exactly one DNS label
// (e.g. "*.intern.example.com" matches "foo.intern.example.com"
// but not "a.b.intern.example.com").
func HostMatches(hostGlob, host string) bool {
	hostGlob = strings.ToLower(strings.TrimSpace(hostGlob))
	host = normalizeHost(host)
	if hostGlob == "" || host == "" {
		return false
	}
	return matchHostGlob(hostGlob, host)
}

// FindServicesForHost returns every configured service whose host glob matches host.
func FindServicesForHost(services map[string]config.ServiceCred, host string) []config.ServiceCred {
	host = normalizeHost(host)
	if host == "" || len(services) == 0 {
		return nil
	}
	matched := make([]config.ServiceCred, 0)
	for _, cred := range services {
		if HostMatches(cred.HostGlob, host) {
			matched = append(matched, cred)
		}
	}
	return matched
}

func matchHostGlob(pattern, host string) bool {
	if pattern == "*" {
		return host != ""
	}
	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")
	if len(patternParts) != len(hostParts) {
		return false
	}
	for i := range patternParts {
		p := patternParts[i]
		h := hostParts[i]
		if p == "*" {
			if h == "" {
				return false
			}
			continue
		}
		if p != h {
			return false
		}
	}
	return true
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	// Strip brackets/port from "[::1]:8080" or "example.com:443".
	if strings.HasPrefix(host, "[") {
		end := strings.IndexByte(host, ']')
		if end < 0 {
			return host
		}
		return host[:end+1]
	}
	if h, _, ok := strings.Cut(host, ":"); ok {
		return h
	}
	return host
}
