package serve

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/auth"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/flood"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/ipwhitelist"
)

// NewHandler builds the HTTP handler used for Caddy forward_auth probes.
// Only exact "/" and "/auth" paths are accepted (README contract).
// A single "/" registration is used because Go's "/" pattern is a catch-all;
// the in-handler path gate rejects every other path with 404.
// whitelist and floodEng may be nil to disable those features (tests).
// When verbose is true, successful probes are logged too; errors and
// rejections are always logged with atomic key=value fields.
func NewHandler(
	logger *log.Logger,
	verbose bool,
	allowedOrigins []string,
	services map[string]auth.ServiceCred,
	realm string,
	whitelist *ipwhitelist.Bundle,
	floodEng *flood.Engine,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", authProbe(logger, verbose, allowedOrigins, services, realm, whitelist, floodEng))
	return mux
}

func authProbe(
	logger *log.Logger,
	verbose bool,
	allowedOrigins []string,
	services map[string]auth.ServiceCred,
	realm string,
	whitelist *ipwhitelist.Bundle,
	floodEng *flood.Engine,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		targetHost := requestTargetHost(r)
		now := time.Now().UTC()
		clientIP := flood.ClientIP(r)
		origin := r.Header.Get("Origin")

		// Exact-path gate: ServeMux "/" is a catch-all; reject other paths with 404.
		if path != "/" && path != "/auth" {
			http.NotFound(w, r)
			logAuthEvent(logger, verbose, http.StatusNotFound, method, path, targetHost, origin, clientIP, "", "", "not_found")
			return
		}

		if !auth.OriginAllowed(origin, allowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			logAuthEvent(logger, verbose, http.StatusForbidden, method, path, targetHost, origin, clientIP, "", "", "origin_not_allowed")
			return
		}

		matched := auth.FindServicesForHost(services, targetHost)
		if len(matched) == 0 {
			unauthorized(w, realm)
			logAuthEvent(logger, verbose, http.StatusUnauthorized, method, path, targetHost, origin, clientIP, "", "", "no_service")
			return
		}
		serviceHint := matched[0].Name

		// Temporary IP whitelist: skip Basic auth and ban enforcement for remembered clients.
		// Checked before bans so active whitelist entries are not blocked or flood-counted.
		if whitelist != nil && clientIP != "" && whitelist.Contains(clientIP, now) {
			whitelist.UpsertNow(clientIP)
			w.WriteHeader(http.StatusOK)
			return
		}

		if floodEng != nil {
			ban, blocked := floodEng.CheckBan(w, r, serviceHint, now)
			if blocked {
				reason := "temp_banned"
				if ban.Permanent {
					reason = "banned"
				}
				logAuthEvent(logger, verbose, http.StatusForbidden, method, path, targetHost, origin, clientIP, serviceHint, "", reason)
				return
			}
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			if floodEng != nil && clientIP != "" && !isWhitelisted(whitelist, clientIP, now) {
				floodEng.RecordFailure(clientIP, serviceHint, now)
			}
			unauthorized(w, realm)
			logAuthEvent(logger, verbose, http.StatusUnauthorized, method, path, targetHost, origin, clientIP, serviceHint, "", "no_credentials")
			return
		}

		cred, ok := auth.CheckBasicAuthAgainstServices(matched, username, password)
		if !ok {
			if floodEng != nil && clientIP != "" && !isWhitelisted(whitelist, clientIP, now) {
				floodEng.RecordFailure(clientIP, serviceHint, now)
			}
			unauthorized(w, realm)
			logAuthEvent(logger, verbose, http.StatusUnauthorized, method, path, targetHost, origin, clientIP, serviceHint, username, "auth_failed")
			return
		}

		if whitelist != nil && clientIP != "" {
			whitelist.UpsertNow(clientIP)
		}

		w.Header().Set("Remote-User", cred.Username)
		w.WriteHeader(http.StatusOK)
		logAuthEvent(logger, verbose, http.StatusOK, method, path, targetHost, origin, clientIP, cred.Name, cred.Username, "ok")
	}
}

// logAuthEvent writes one atomic key=value auth line.
// Errors and rejections (status >= 400) always log; successes only when verbose.
func isWhitelisted(whitelist *ipwhitelist.Bundle, ip string, now time.Time) bool {
	return whitelist != nil && ip != "" && whitelist.Contains(ip, now)
}

func logAuthEvent(
	logger *log.Logger,
	verbose bool,
	status int,
	method, path, host, origin, ip, service, user, reason string,
) {
	if status < 400 && !verbose {
		return
	}
	logger.Printf(
		"auth status=%d method=%s path=%s host=%s origin=%s ip=%s service=%s user=%s reason=%s",
		status,
		atom(method),
		atom(path),
		atom(host),
		atom(origin),
		atom(ip),
		atom(service),
		atom(user),
		atom(reason),
	)
}

// atom formats a log field value: empty -> "-", otherwise the raw value, or
// a quoted form when the value contains whitespace or quotes.
func atom(value string) string {
	if value == "" {
		return "-"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '"'
	}) >= 0 {
		return strconv.Quote(value)
	}
	return value
}

func requestTargetHost(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); h != "" {
		// X-Forwarded-Host may be a CSV; use the first value.
		if i := strings.IndexByte(h, ','); i >= 0 {
			h = h[:i]
		}
		return strings.TrimSpace(h)
	}
	return r.Host
}

func unauthorized(w http.ResponseWriter, realm string) {
	if realm == "" {
		realm = "restricted"
	}
	realm = strings.ReplaceAll(realm, `\`, `\\`)
	realm = strings.ReplaceAll(realm, `"`, `\"`)
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
