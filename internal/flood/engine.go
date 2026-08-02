package flood

import (
	"fmt"
	"sync"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodban"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodtrack"
)

// DefaultCleanupInterval is how often retention cleanup runs.
const DefaultCleanupInterval = time.Hour

// Engine ties flood tracking and ban storage into record / punish / enforce /
// cleanup helpers. It does not start HTTP serving by itself.
type Engine struct {
	Track *floodtrack.Bundle
	Bans  *floodban.Bundle

	mu             sync.Mutex
	cleanupStopCh  chan struct{}
	cleanupDoneCh  chan struct{}
	cleanupRunning bool
}

// OpenDefaults loads track and ban bundles from their default paths.
func OpenDefaults() (*Engine, error) {
	track, err := floodtrack.OpenDefault()
	if err != nil {
		return nil, fmt.Errorf("flood: track: %w", err)
	}
	bans, err := floodban.OpenDefault()
	if err != nil {
		return nil, fmt.Errorf("flood: bans: %w", err)
	}
	return &Engine{Track: track, Bans: bans}, nil
}

// Open loads track and ban bundles from the given paths (empty uses defaults).
func Open(trackPath, banPath string) (*Engine, error) {
	track, err := floodtrack.Open(trackPath)
	if err != nil {
		return nil, fmt.Errorf("flood: track: %w", err)
	}
	bans, err := floodban.Open(banPath)
	if err != nil {
		return nil, fmt.Errorf("flood: bans: %w", err)
	}
	return &Engine{Track: track, Bans: bans}, nil
}
