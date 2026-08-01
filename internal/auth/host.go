package auth

import (
	"sort"
	"strings"
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
func FindServicesForHost(services map[string]ServiceCred, host string) []ServiceCred {
	if len(services) == 0 {
		return nil
	}
	matched := make([]ServiceCred, 0)
	for _, cred := range services {
		if HostMatches(cred.HostGlob, host) {
			matched = append(matched, cred)
		}
	}
	return matched
}

// HostGlobsOverlap reports whether two host globs can both match the same host
// under HostMatches rules (case-insensitive; '*' is one DNS label; bare '*' is any host).
func HostGlobsOverlap(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == "*" || b == "*" {
		return true
	}
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	if len(aParts) != len(bParts) {
		return false
	}
	for i := range aParts {
		if !labelsCompatible(aParts[i], bParts[i]) {
			return false
		}
	}
	return true
}

// OverlappingHostGlobPairs returns sorted service-name pairs whose host globs overlap.
func OverlappingHostGlobPairs(services map[string]ServiceCred) [][2]string {
	if len(services) < 2 {
		return nil
	}
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)

	pairs := make([][2]string, 0)
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			left, right := names[i], names[j]
			if HostGlobsOverlap(services[left].HostGlob, services[right].HostGlob) {
				pairs = append(pairs, [2]string{left, right})
			}
		}
	}
	return pairs
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
		if !labelMatch(patternParts[i], hostParts[i]) {
			return false
		}
	}
	return true
}

// labelMatch reports whether a pattern label matches a host label (* = one non-empty label).
func labelMatch(patternLabel, hostLabel string) bool {
	if patternLabel == "*" {
		return hostLabel != ""
	}
	return patternLabel == hostLabel
}

// labelsCompatible reports whether two pattern labels can both match some host label.
func labelsCompatible(a, b string) bool {
	if a == "*" || b == "*" {
		return true
	}
	return a == b
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
