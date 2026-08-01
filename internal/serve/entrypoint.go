package serve

import (
	"fmt"
	"log"
	"net/http"

	"github.com/NobleMajo/intern-auth-gateway/internal/auth"
	"github.com/NobleMajo/intern-auth-gateway/internal/config"
)

// Run starts the HTTP server for caddy forward_auth probes.
func Run(logger *log.Logger, appConfig *config.AppConfig) error {
	services, err := auth.LoadServicesFromEnv()
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", appConfig.Host, appConfig.Port)
	origins := appConfig.AllowedOriginList()

	logger.Printf(
		"starting intern-auth-gateway on %s (origins=%d services=%d)",
		addr,
		len(origins),
		len(services),
	)
	if appConfig.Verbose {
		for name, cred := range services {
			logger.Printf("service %s -> hostGlob=%s user=%s", name, cred.HostGlob, cred.Username)
		}
		for _, origin := range origins {
			logger.Printf("allowed origin %s", origin)
		}
	}

	handler := NewHandler(logger, appConfig, services)
	return http.ListenAndServe(addr, handler)
}
