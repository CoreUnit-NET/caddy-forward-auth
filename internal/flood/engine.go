package flood

import (
	"fmt"
	"sync"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodban"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodtrack"
)

// DefaultCleanupInterval is how often retention cleanup runs when Options.CleanupInterval is zero.
const DefaultCleanupInterval = time.Hour

// Engine ties flood tracking and ban storage into record / punish / enforce /
// cleanup helpers. It does not start HTTP serving by itself.
type Engine struct {
	Track *floodtrack.Bundle
	Bans  *floodban.Bundle

	rules              []Rule
	retention          time.Duration
	countTempBanProbes bool

	mu             sync.Mutex
	cleanupStopCh  chan struct{}
	cleanupDoneCh  chan struct{}
	cleanupRunning bool
}

// OpenDefaults loads track and ban bundles with DefaultOptions.
func OpenDefaults() (*Engine, error) {
	return OpenWithOptions(DefaultOptions())
}

// Open loads track and ban bundles from the given paths (empty uses defaults).
func Open(trackPath, banPath string) (*Engine, error) {
	opts := DefaultOptions()
	opts.FloodPath = trackPath
	opts.BanPath = banPath
	return OpenWithOptions(opts)
}

// OpenWithOptions loads bundles and applies runtime flood options.
func OpenWithOptions(opts Options) (*Engine, error) {
	if opts.FloodPath == "" {
		opts.FloodPath = DefaultOptions().FloodPath
	}
	if opts.BanPath == "" {
		opts.BanPath = DefaultOptions().BanPath
	}
	if len(opts.Rules) == 0 {
		opts.Rules = defaultRules()
	}
	if opts.Retention <= 0 {
		opts.Retention = DefaultOptions().Retention
	}

	track, err := floodtrack.Open(opts.FloodPath)
	if err != nil {
		return nil, fmt.Errorf("flood: track: %w", err)
	}
	bans, err := floodban.Open(opts.BanPath)
	if err != nil {
		return nil, fmt.Errorf("flood: bans: %w", err)
	}
	return &Engine{
		Track:              track,
		Bans:               bans,
		rules:              append([]Rule(nil), opts.Rules...),
		retention:          opts.Retention,
		countTempBanProbes: opts.CountTempBanProbes,
	}, nil
}

// ClearFailures removes all flood tracking entries for ip.
func (e *Engine) ClearFailures(ip string) {
	if e == nil || e.Track == nil || ip == "" {
		return
	}
	e.Track.RemoveForIP(ip)
}
