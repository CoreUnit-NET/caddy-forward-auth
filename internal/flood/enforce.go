package flood

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/floodban"
)

// ClientIP resolves the remote client address for flood tracking.
// Order: first X-Forwarded-For hop, then X-Real-IP, then RemoteAddr.
// Intended for a private network behind a trusted reverse proxy (Caddy).
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return stripIPBrackets(ip)
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return stripIPBrackets(xri)
	}
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return stripIPBrackets(strings.TrimSpace(host))
}

func stripIPBrackets(ip string) string {
	if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") && len(ip) > 2 {
		return ip[1 : len(ip)-1]
	}
	return ip
}

// CheckBan enforces permanent and temporary bans for the request client IP.
// Permanent bans return 403 immediately without updating flood tracking.
// Temporary bans still record a failure (and may escalate) then return 403.
// service may be empty when the target service is not known yet.
// ok is false when the request is blocked.
func (e *Engine) CheckBan(w http.ResponseWriter, r *http.Request, service string, now time.Time) (ban floodban.Ban, blocked bool) {
	if e == nil || e.Bans == nil || w == nil || r == nil {
		return floodban.Ban{}, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ip := ClientIP(r)
	if ip == "" {
		return floodban.Ban{}, false
	}
	found, ok := e.Bans.IsBanned(ip, now)
	if !ok {
		return floodban.Ban{}, false
	}
	if found.Permanent {
		http.Error(w, "banned", http.StatusForbidden)
		return found, true
	}
	// Temp bans still count as flood violations.
	e.RecordFailure(ip, service, now)
	http.Error(w, "temporarily banned", http.StatusForbidden)
	return found, true
}
