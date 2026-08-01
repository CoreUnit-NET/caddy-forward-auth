package serve

import (
	"fmt"
	"log"
	"net/http"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/auth"
	"github.com/CoreUnit-NET/intern-auth-gateway/internal/config"
)

// Run starts the HTTP server for caddy forward_auth probes.
func Run(logger *log.Logger, shortName string, appConfig *config.AppConfig) error {
	services, err := auth.LoadServicesFromEnv()
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", appConfig.Host, appConfig.Port)
	origins := appConfig.AllowedOriginList()

	logger.Printf(
		"starting %s on %s (origins=%d services=%d)",
		shortName,
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

	handler := NewHandler(logger, appConfig.Verbose, origins, services, shortName)
	return http.ListenAndServe(addr, handler)
}
