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
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/settings"
)

// NewHandler builds the HTTP handler used for Caddy forward_auth probes.
// Only exact "/" and "/auth" paths are accepted (README contract).
// A single "/" registration is used because Go's "/" pattern is a catch-all;
// the in-handler path gate rejects every other path with 404.
// cfg may be nil (tests); whitelist and floodEng may be nil to disable those features.
func NewHandler(
	logger *log.Logger,
	cfg *settings.Settings,
	services map[string]auth.ServiceCred,
	realm string,
	whitelist *ipwhitelist.Bundle,
	floodEng *flood.Engine,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", authProbe(logger, cfg, services, realm, whitelist, floodEng))
	return mux
}

func authProbe(
	logger *log.Logger,
	cfg *settings.Settings,
	services map[string]auth.ServiceCred,
	realm string,
	whitelist *ipwhitelist.Bundle,
	floodEng *flood.Engine,
) http.HandlerFunc {
	logSuccess := cfg != nil && cfg.LogSuccessfulAuth()

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		targetHost := requestTargetHost(r)
		now := time.Now().UTC()
		clientIP := flood.ClientIP(r)
		origin := r.Header.Get("Origin")
		allowedOrigins := allowedOriginsFrom(cfg)

		// Exact-path gate: ServeMux "/" is a catch-all; reject other paths with 404.
		if path != "/" && path != "/auth" {
			http.NotFound(w, r)
			logAuthEvent(logger, logSuccess, http.StatusNotFound, method, path, targetHost, origin, clientIP, "", "", "not_found")
			return
		}

		if !auth.OriginAllowed(origin, allowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			logAuthEvent(logger, logSuccess, http.StatusForbidden, method, path, targetHost, origin, clientIP, "", "", "origin_not_allowed")
			return
		}

		matched := auth.FindServicesForHost(services, targetHost)
		if len(matched) == 0 {
			unauthorized(w, realm)
			logAuthEvent(logger, logSuccess, http.StatusUnauthorized, method, path, targetHost, origin, clientIP, "", "", "no_service")
			return
		}
		serviceHint := matched[0].Name

		whitelistActive := cfg != nil && cfg.Whitelist.Enabled && whitelist != nil
		whitelisted := whitelistActive && isWhitelisted(whitelist, clientIP, now)
		if whitelisted {
			whitelist.UpsertNow(clientIP)
			if floodEng != nil && cfg != nil && cfg.Flood.Enabled && cfg.Flood.ClearOnWhitelist {
				floodEng.ClearFailures(clientIP)
			}
		}

		skipBan := whitelisted && cfg != nil && cfg.Whitelist.OverridesBan
		if floodEng != nil && cfg != nil && cfg.Flood.Enabled && !skipBan {
			ban, blocked := floodEng.CheckBan(w, r, serviceHint, now)
			if blocked {
				reason := "temp_banned"
				if ban.Permanent {
					reason = "banned"
				}
				logAuthEvent(logger, logSuccess, http.StatusForbidden, method, path, targetHost, origin, clientIP, serviceHint, "", reason)
				return
			}
		}

		if whitelisted {
			w.WriteHeader(http.StatusOK)
			if cfg != nil && cfg.LogWhitelistedProbe() {
				logAuthEvent(logger, true, http.StatusOK, method, path, targetHost, origin, clientIP, serviceHint, "", "whitelisted")
			}
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			if floodEng != nil && cfg != nil && cfg.Flood.Enabled && clientIP != "" && !whitelisted {
				if cfg.Flood.CountNoCredentials {
					floodEng.RecordFailure(clientIP, serviceHint, now)
				}
			}
			unauthorized(w, realm)
			logAuthEvent(logger, logSuccess, http.StatusUnauthorized, method, path, targetHost, origin, clientIP, serviceHint, "", "no_credentials")
			return
		}

		cred, ok := auth.CheckBasicAuthAgainstServices(matched, username, password)
		if !ok {
			if floodEng != nil && cfg != nil && cfg.Flood.Enabled && clientIP != "" && !whitelisted {
				floodEng.RecordFailure(clientIP, serviceHint, now)
			}
			unauthorized(w, realm)
			logAuthEvent(logger, logSuccess, http.StatusUnauthorized, method, path, targetHost, origin, clientIP, serviceHint, username, "auth_failed")
			return
		}

		if whitelistActive && clientIP != "" {
			whitelist.UpsertNow(clientIP)
		}

		w.Header().Set("Remote-User", cred.Username)
		w.WriteHeader(http.StatusOK)
		logAuthEvent(logger, logSuccess, http.StatusOK, method, path, targetHost, origin, clientIP, cred.Name, cred.Username, "ok")
	}
}

func allowedOriginsFrom(cfg *settings.Settings) []string {
	if cfg == nil {
		return nil
	}
	return cfg.AllowedOrigins
}

// logAuthEvent writes one atomic key=value auth line.
// Errors and rejections (status >= 400) always log; successes only when logSuccess.
func isWhitelisted(whitelist *ipwhitelist.Bundle, ip string, now time.Time) bool {
	return whitelist != nil && ip != "" && whitelist.Contains(ip, now)
}

func logAuthEvent(
	logger *log.Logger,
	logSuccess bool,
	status int,
	method, path, host, origin, ip, service, user, reason string,
) {
	if status < 400 && !logSuccess {
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
