package serve

import (
	"log"
	"net/http"
	"strings"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/auth"
)

// NewHandler builds the HTTP handler used for caddy forward_auth probes.
// Only exact "/" and "/auth" paths are registered (README contract).
// The in-handler path gate is required because Go's "/" pattern is a catch-all.
func NewHandler(logger *log.Logger, verbose bool, allowedOrigins []string, services map[string]auth.ServiceCred, realm string) http.Handler {
	mux := http.NewServeMux()
	probe := authProbe(logger, verbose, allowedOrigins, services, realm)
	mux.HandleFunc("/", probe)
	mux.HandleFunc("/auth", probe)
	return mux
}

func authProbe(logger *log.Logger, verbose bool, allowedOrigins []string, services map[string]auth.ServiceCred, realm string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Exact-path gate: ServeMux "/" is a catch-all; reject other paths with 404.
		if r.URL.Path != "/" && r.URL.Path != "/auth" {
			http.NotFound(w, r)
			return
		}

		if !auth.OriginAllowed(r.Header.Get("Origin"), allowedOrigins) {
			if verbose {
				logger.Printf("origin rejected: %q", r.Header.Get("Origin"))
			}
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		targetHost := requestTargetHost(r)
		matched := auth.FindServicesForHost(services, targetHost)
		if len(matched) == 0 {
			if verbose {
				logger.Printf("no service for host %q", targetHost)
			}
			unauthorized(w, realm)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			unauthorized(w, realm)
			return
		}

		cred, ok := auth.CheckBasicAuthAgainstServices(matched, username, password)
		if !ok {
			if verbose {
				logger.Printf("basic auth failed for host %q user %q", targetHost, username)
			}
			unauthorized(w, realm)
			return
		}

		if verbose {
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

func unauthorized(w http.ResponseWriter, realm string) {
	if realm == "" {
		realm = "restricted"
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
