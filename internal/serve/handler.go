package serve

import (
	"log"
	"net/http"
	"strings"

	"github.com/NobleMajo/intern-auth-gateway/internal/auth"
	"github.com/NobleMajo/intern-auth-gateway/internal/config"
)

const basicRealm = "intern-auth-gateway"

// NewHandler builds the HTTP handler used for caddy forward_auth probes.
func NewHandler(logger *log.Logger, appConfig *config.AppConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", authProbe(logger, appConfig))
	mux.HandleFunc("/auth", authProbe(logger, appConfig))
	return mux
}

func authProbe(logger *log.Logger, appConfig *config.AppConfig) http.HandlerFunc {
	allowedOrigins := appConfig.AllowedOriginList()

	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.OriginAllowed(r.Header.Get("Origin"), allowedOrigins) {
			if appConfig.Verbose {
				logger.Printf("origin rejected: %q", r.Header.Get("Origin"))
			}
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		targetHost := requestTargetHost(r)
		matched := auth.FindServicesForHost(appConfig.Services, targetHost)
		if len(matched) == 0 {
			if appConfig.Verbose {
				logger.Printf("no service for host %q", targetHost)
			}
			unauthorized(w)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			unauthorized(w)
			return
		}

		cred, ok := auth.CheckBasicAuthAgainstServices(matched, username, password)
		if !ok {
			if appConfig.Verbose {
				logger.Printf("basic auth failed for host %q user %q", targetHost, username)
			}
			unauthorized(w)
			return
		}

		if appConfig.Verbose {
			logger.Printf("authorized host %q user %q", targetHost, cred.Username)
		}
		w.Header().Set("Remote-User", cred.Username)
		w.WriteHeader(http.StatusOK)
	}
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

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+basicRealm+`"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
