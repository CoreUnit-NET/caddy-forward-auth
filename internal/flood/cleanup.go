package flood

import (
	"errors"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodtrack"
)

// ErrCleanupRunning is returned when StartCleanup is called while a cleanup
// loop is already active.
var ErrCleanupRunning = errors.New("flood: cleanup already running")

// ErrCleanupNotRunning is returned when StopCleanup is called while no cleanup
// loop is active.
var ErrCleanupNotRunning = errors.New("flood: cleanup not running")

// Cleanup removes flood entries older than floodtrack.DefaultRetention and
// expired temporary bans. It marks bundles dirty when something changes.
func (e *Engine) Cleanup(now time.Time) {
	if e == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if e.Track != nil {
		cutoff := now.Add(-e.retention)
		if e.retention <= 0 {
			cutoff = now.Add(-floodtrack.DefaultRetention)
		}
		e.Track.RemoveOlderThan(cutoff)
	}
	if e.Bans != nil {
		e.Bans.RemoveExpired(now)
	}
}

// StartCleanup runs Cleanup once immediately, then on interval.
// Non-positive interval uses DefaultCleanupInterval.
// Only one cleanup loop is allowed at a time per Engine.
func (e *Engine) StartCleanup(interval time.Duration) error {
	if e == nil {
		return errors.New("flood: nil engine")
	}
	if interval <= 0 {
		interval = DefaultCleanupInterval
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanupRunning {
		return ErrCleanupRunning
	}
	e.cleanupStopCh = make(chan struct{})
	e.cleanupDoneCh = make(chan struct{})
	e.cleanupRunning = true
	stopCh := e.cleanupStopCh
	doneCh := e.cleanupDoneCh
	e.Cleanup(time.Now().UTC())
	go e.cleanupLoop(interval, stopCh, doneCh)
	return nil
}

// StopCleanup stops the background cleanup loop and waits for it to finish.
func (e *Engine) StopCleanup() error {
	if e == nil {
		return errors.New("flood: nil engine")
	}
	e.mu.Lock()
	if !e.cleanupRunning {
		e.mu.Unlock()
		return ErrCleanupNotRunning
	}
	stopCh := e.cleanupStopCh
	doneCh := e.cleanupDoneCh
	e.mu.Unlock()

	close(stopCh)
	<-doneCh

	e.mu.Lock()
	defer e.mu.Unlock()
	e.cleanupRunning = false
	e.cleanupStopCh = nil
	e.cleanupDoneCh = nil
	return nil
}

// CleanupRunning reports whether the cleanup loop is active.
func (e *Engine) CleanupRunning() bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cleanupRunning
}

func (e *Engine) cleanupLoop(interval time.Duration, stopCh, doneCh chan struct{}) {
	defer close(doneCh)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			e.Cleanup(time.Now().UTC())
		}
	}
}

// StartPersistence starts periodic dirty saves for track and ban bundles and
// starts hourly (by default) cleanup. Non-positive saveInterval uses each
// bundle default. Non-positive cleanupInterval uses DefaultCleanupInterval.
func (e *Engine) StartPersistence(saveInterval, cleanupInterval time.Duration) error {
	if e == nil || e.Track == nil || e.Bans == nil {
		return errors.New("flood: engine not initialized")
	}
	if err := e.Track.StartPeriodicSave(saveInterval); err != nil {
		return err
	}
	if err := e.Bans.StartPeriodicSave(saveInterval); err != nil {
		_ = e.Track.StopPeriodicSave()
		return err
	}
	if err := e.StartCleanup(cleanupInterval); err != nil {
		_ = e.Bans.StopPeriodicSave()
		_ = e.Track.StopPeriodicSave()
		return err
	}
	return nil
}

// StopPersistence stops cleanup and both periodic savers (flushing dirty state).
func (e *Engine) StopPersistence() error {
	if e == nil {
		return errors.New("flood: nil engine")
	}
	var first error
	if e.CleanupRunning() {
		if err := e.StopCleanup(); err != nil && first == nil {
			first = err
		}
	}
	if e.Bans != nil && e.Bans.PeriodicSaveRunning() {
		if err := e.Bans.StopPeriodicSave(); err != nil && first == nil {
			first = err
		}
	}
	if e.Track != nil && e.Track.PeriodicSaveRunning() {
		if err := e.Track.StopPeriodicSave(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
