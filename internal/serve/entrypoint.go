package serve

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/auth"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/config"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/flood"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/ipwhitelist"
)

// Run starts the HTTP server for Caddy forward_auth probes.
func Run(logger *log.Logger, shortName string, appConfig *config.AppConfig) error {
	services, err := auth.LoadServicesFromEnv()
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}

	whitelist, err := ipwhitelist.OpenDefault()
	if err != nil {
		return fmt.Errorf("ip whitelist: %w", err)
	}
	if err := whitelist.StartPeriodicSave(0); err != nil {
		return fmt.Errorf("ip whitelist: start save: %w", err)
	}

	floodEng, err := flood.OpenDefaults()
	if err != nil {
		_ = whitelist.StopPeriodicSave()
		return fmt.Errorf("flood: %w", err)
	}
	if err := floodEng.StartPersistence(0, 0); err != nil {
		_ = whitelist.StopPeriodicSave()
		return fmt.Errorf("flood: start persistence: %w", err)
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
	for _, pair := range auth.OverlappingHostGlobPairs(services) {
		left, right := pair[0], pair[1]
		logger.Printf(
			"warning: overlapping host globs for SERVICE_%s (%s) and SERVICE_%s (%s)",
			left,
			services[left].HostGlob,
			right,
			services[right].HostGlob,
		)
	}
	if appConfig.Verbose {
		logVerboseConfig(logger, services, origins)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           NewHandler(logger, origins, services, shortName, whitelist, floodEng),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Printf("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("http shutdown: %v", err)
		}
		if err := floodEng.StopPersistence(); err != nil {
			logger.Printf("flood stop: %v", err)
		}
		if err := whitelist.StopPeriodicSave(); err != nil {
			logger.Printf("ip whitelist stop: %v", err)
		}
		return nil
	case err := <-errCh:
		_ = floodEng.StopPersistence()
		_ = whitelist.StopPeriodicSave()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func logVerboseConfig(logger *log.Logger, services map[string]auth.ServiceCred, origins []string) {
	for _, name := range auth.SortedServiceNames(services) {
		cred := services[name]
		logger.Printf(
			"service %s -> hostGlob=%s user=%s passwordHash=%s",
			name,
			cred.HostGlob,
			cred.Username,
			cred.PasswordHash,
		)
	}
	for _, origin := range origins {
		logger.Printf("allowed origin %s", origin)
	}
}
