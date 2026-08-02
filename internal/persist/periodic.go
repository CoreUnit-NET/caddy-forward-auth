package persist

import (
	"errors"
	"log"
	"sync"
	"time"
)

// ErrAlreadyRunning is returned when Start is called while a saver is active.
var ErrAlreadyRunning = errors.New("persist: periodic save already running")

// ErrNotRunning is returned when Stop is called while no saver is active.
var ErrNotRunning = errors.New("persist: periodic save not running")

// Periodic runs a single background loop that invokes save on an interval.
// Only one loop is allowed at a time.
type Periodic struct {
	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
}

// Start begins the background loop. Non-positive interval uses defaultInterval
// when defaultInterval is positive; otherwise interval must be positive.
// save is called from the ticker (and should be safe to call concurrently with Stop's flush).
func (p *Periodic) Start(interval, defaultInterval time.Duration, save func() error, logPrefix string) error {
	if interval <= 0 {
		interval = defaultInterval
	}
	if interval <= 0 {
		return errors.New("persist: non-positive save interval")
	}
	if save == nil {
		return errors.New("persist: nil save func")
	}
	if logPrefix == "" {
		logPrefix = "persist"
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return ErrAlreadyRunning
	}
	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})
	p.running = true
	stopCh := p.stopCh
	doneCh := p.doneCh
	go periodicLoop(interval, stopCh, doneCh, save, logPrefix)
	return nil
}

// Stop ends the background loop, waits for it to finish, then runs flush once.
func (p *Periodic) Stop(flush func() error) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return ErrNotRunning
	}
	stopCh := p.stopCh
	doneCh := p.doneCh
	p.mu.Unlock()

	close(stopCh)
	<-doneCh

	p.mu.Lock()
	p.running = false
	p.stopCh = nil
	p.doneCh = nil
	p.mu.Unlock()

	if flush == nil {
		return nil
	}
	return flush()
}

// Running reports whether the periodic saver is active.
func (p *Periodic) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func periodicLoop(interval time.Duration, stopCh, doneCh chan struct{}, save func() error, logPrefix string) {
	defer close(doneCh)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if err := save(); err != nil {
				log.Printf("%s: periodic save: %v", logPrefix, err)
			}
		}
	}
}
