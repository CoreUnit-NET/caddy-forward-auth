package auth

import (
	"net/url"
	"strings"
)

// OriginAllowed reports whether the request Origin header is permitted.
// Rules:
//   - empty allowed list => no Origin enforcement (always true)
//   - empty Origin header => allowed (non-browser / caddy probes)
//   - otherwise the Origin URL hostname must be in allowed (case-insensitive)
func OriginAllowed(originHeader string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	originHeader = strings.TrimSpace(originHeader)
	if originHeader == "" {
		return true
	}
	host, ok := originHostname(originHeader)
	if !ok {
		return false
	}
	for _, item := range allowed {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" && item == host {
			return true
		}
	}
	return false
}

func originHostname(origin string) (string, bool) {
	// Origin is normally an absolute URL (scheme://host[:port]).
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		// Fallback: treat bare hostname values as the host.
		host := normalizeHost(origin)
		return host, host != ""
	}
	host := normalizeHost(u.Host)
	return host, host != ""
}
