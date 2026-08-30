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
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/settings"
)

// Run starts the HTTP server for Caddy forward_auth probes.
func Run(logger *log.Logger, shortName string, appConfig *config.AppConfig) error {
	st, err := settings.FromAppConfig(appConfig)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}

	services, err := auth.LoadServicesFromEnv()
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}

	var whitelist *ipwhitelist.Bundle
	if st.Whitelist.Enabled {
		whitelist, err = ipwhitelist.OpenWithPeriod(st.Whitelist.Path, st.WhitelistPeriod())
		if err != nil {
			return fmt.Errorf("ip whitelist: %w", err)
		}
		if err := whitelist.StartPeriodicSave(st.DataSaveInterval()); err != nil {
			return fmt.Errorf("ip whitelist: start save: %w", err)
		}
	}

	var floodEng *flood.Engine
	if st.Flood.Enabled {
		floodEng, err = flood.OpenWithOptions(flood.OptionsFromSettings(st))
		if err != nil {
			stopWhitelist(whitelist)
			return fmt.Errorf("flood: %w", err)
		}
		if err := floodEng.StartPersistence(st.DataSaveInterval(), st.FloodCleanupInterval()); err != nil {
			stopWhitelist(whitelist)
			return fmt.Errorf("flood: start persistence: %w", err)
		}
	}

	addr := fmt.Sprintf("%s:%d", st.Host, st.Port)

	logger.Printf(
		"starting %s on %s (origins=%d services=%d whitelist=%t flood=%t)",
		shortName,
		addr,
		len(st.AllowedOrigins),
		len(services),
		st.Whitelist.Enabled,
		st.Flood.Enabled,
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
	if st.Verbose {
		logVerboseConfig(logger, st, services)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           NewHandler(logger, st, services, shortName, whitelist, floodEng),
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
		stopFlood(floodEng)
		stopWhitelist(whitelist)
		return nil
	case err := <-errCh:
		stopFlood(floodEng)
		stopWhitelist(whitelist)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func stopWhitelist(whitelist *ipwhitelist.Bundle) {
	if whitelist == nil {
		return
	}
	if err := whitelist.StopPeriodicSave(); err != nil {
		// best-effort on shutdown
	}
}

func stopFlood(floodEng *flood.Engine) {
	if floodEng == nil {
		return
	}
	if err := floodEng.StopPersistence(); err != nil {
		// best-effort on shutdown
	}
}

func logVerboseConfig(logger *log.Logger, st *settings.Settings, services map[string]auth.ServiceCred) {
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
	for _, origin := range st.AllowedOrigins {
		logger.Printf("allowed origin %s", origin)
	}
	if st.Whitelist.Enabled {
		logger.Printf(
			"whitelist path=%s periodHours=%d overridesBan=%t",
			st.Whitelist.Path,
			st.Whitelist.PeriodHours,
			st.Whitelist.OverridesBan,
		)
	}
	if st.Flood.Enabled {
		logger.Printf(
			"flood paths track=%s ban=%s retentionHours=%d cleanupMins=%d saveSecs=%d clearOnWhitelist=%t countNoCredentials=%t countTempBanProbes=%t tiers=%d",
			st.Flood.FloodPath,
			st.Flood.BanPath,
			st.Flood.RetentionHours,
			st.Flood.CleanupMins,
			st.Flood.SaveSecs,
			st.Flood.ClearOnWhitelist,
			st.Flood.CountNoCredentials,
			st.Flood.CountTempBanProbes,
			len(st.Flood.Tiers),
		)
	}
}
