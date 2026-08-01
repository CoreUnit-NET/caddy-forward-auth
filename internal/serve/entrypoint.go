package serve

import (
	"fmt"
	"log"
	"net/http"

	"github.com/NobleMajo/intern-auth-gateway/internal/config"
)

// Run starts the HTTP server for caddy forward_auth probes.
func Run(logger *log.Logger, appConfig *config.AppConfig) error {
	addr := fmt.Sprintf("%s:%d", appConfig.Host, appConfig.Port)
	origins := appConfig.AllowedOriginList()

	logger.Printf(
		"starting intern-auth-gateway on %s (origins=%d services=%d)",
		addr,
		len(origins),
		len(appConfig.Services),
	)
	if appConfig.Verbose {
		for name, cred := range appConfig.Services {
			logger.Printf("service %s -> hostGlob=%s user=%s", name, cred.HostGlob, cred.Username)
		}
		for _, origin := range origins {
			logger.Printf("allowed origin %s", origin)
		}
	}

	handler := NewHandler(logger, appConfig)
	return http.ListenAndServe(addr, handler)
}
