package serve

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/auth"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/flood"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/ipwhitelist"
)

// NewHandler builds the HTTP handler used for caddy forward_auth probes.
// Only exact "/" and "/auth" paths are accepted (README contract).
// A single "/" registration is used because Go's "/" pattern is a catch-all;
// the in-handler path gate rejects every other path with 404.
// whitelist and floodEng may be nil to disable those features (tests).
func NewHandler(
	logger *log.Logger,
	allowedOrigins []string,
	services map[string]auth.ServiceCred,
	realm string,
	whitelist *ipwhitelist.Bundle,
	floodEng *flood.Engine,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", authProbe(logger, allowedOrigins, services, realm, whitelist, floodEng))
	return mux
}

func authProbe(
	logger *log.Logger,
	allowedOrigins []string,
	services map[string]auth.ServiceCred,
	realm string,
	whitelist *ipwhitelist.Bundle,
	floodEng *flood.Engine,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		targetHost := requestTargetHost(r)
		now := time.Now().UTC()
		clientIP := flood.ClientIP(r)

		// Exact-path gate: ServeMux "/" is a catch-all; reject other paths with 404.
		if path != "/" && path != "/auth" {
			http.NotFound(w, r)
			logAuthEvent(logger, http.StatusNotFound, path, targetHost, "", "", "not_found")
			return
		}

		origin := r.Header.Get("Origin")
		if !auth.OriginAllowed(origin, allowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			logAuthEvent(logger, http.StatusForbidden, path, targetHost, "", "", "origin")
			return
		}

		matched := auth.FindServicesForHost(services, targetHost)
		if len(matched) == 0 {
			unauthorized(w, realm)
			logAuthEvent(logger, http.StatusUnauthorized, path, targetHost, "", "", "no_service")
			return
		}
		serviceHint := matched[0].Name

		// Ban check before whitelist/auth. Permanent bans short-circuit;
		// temp bans still record flood failures inside CheckBan.
		if floodEng != nil {
			ban, blocked := floodEng.CheckBan(w, r, serviceHint, now)
			if blocked {
				reason := "temp_banned"
				if ban.Permanent {
					reason = "banned"
				}
				logAuthEvent(logger, http.StatusForbidden, path, targetHost, serviceHint, "", reason)
				return
			}
		}

		// Temporary IP whitelist: skip Basic auth for remembered clients.
		if whitelist != nil && clientIP != "" && whitelist.Contains(clientIP, now) {
			w.WriteHeader(http.StatusOK)
			logAuthEvent(logger, http.StatusOK, path, targetHost, serviceHint, "", "whitelisted")
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			if floodEng != nil && clientIP != "" {
				floodEng.RecordFailure(clientIP, serviceHint, now)
			}
			unauthorized(w, realm)
			logAuthEvent(logger, http.StatusUnauthorized, path, targetHost, serviceHint, "", "no_credentials")
			return
		}

		cred, ok := auth.CheckBasicAuthAgainstServices(matched, username, password)
		if !ok {
			if floodEng != nil && clientIP != "" {
				floodEng.RecordFailure(clientIP, serviceHint, now)
			}
			unauthorized(w, realm)
			logAuthEvent(logger, http.StatusUnauthorized, path, targetHost, serviceHint, username, "auth_failed")
			return
		}

		if whitelist != nil && clientIP != "" {
			whitelist.UpsertNow(clientIP)
		}

		w.Header().Set("Remote-User", cred.Username)
		w.WriteHeader(http.StatusOK)
		logAuthEvent(logger, http.StatusOK, path, targetHost, cred.Name, cred.Username, "ok")
	}
}

func logAuthEvent(logger *log.Logger, status int, path, host, service, user, reason string) {
	logger.Printf(
		"auth status=%d path=%s host=%s service=%s user=%s reason=%s",
		status,
		dashIfEmpty(path),
		dashIfEmpty(host),
		dashIfEmpty(service),
		dashIfEmpty(user),
		dashIfEmpty(reason),
	)
}

func dashIfEmpty(value string) string {
	if value == "" {
		return "-"
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
