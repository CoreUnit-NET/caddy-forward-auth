package ipwhitelist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrPeriodicSaveRunning is returned when StartPeriodicSave is called while a
// saver is already active. Only one period is allowed at a time.
var ErrPeriodicSaveRunning = errors.New("ipwhitelist: periodic save already running")

// ErrPeriodicSaveNotRunning is returned when StopPeriodicSave is called while
// no saver is active.
var ErrPeriodicSaveNotRunning = errors.New("ipwhitelist: periodic save not running")

// Bundle holds whitelist elements and persists them as JSON on disk.
// Mutations mark the bundle dirty; disk writes happen on Save or via the
// periodic saver when dirty (not immediately on each whitelist change).
type Bundle struct {
	mu       sync.Mutex
	path     string
	dirty    bool
	elements []Element

	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
}

// bundleFile is the on-disk JSON shape.
type bundleFile struct {
	Elements []Element `json:"elements"`
}

// OpenDefault loads a Bundle from DefaultPath (./data/ipwhitelist.json).
func OpenDefault() (*Bundle, error) {
	return Open(DefaultPath)
}

// Open loads a Bundle from path. An empty path uses DefaultPath.
// If the file does not exist, an empty bundle bound to that path is returned
// (not marked dirty).
func Open(path string) (*Bundle, error) {
	if path == "" {
		path = DefaultPath
	}
	b := &Bundle{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return nil, fmt.Errorf("ipwhitelist: load %q: %w", path, err)
	}
	if len(data) == 0 {
		return b, nil
	}
	var file bundleFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("ipwhitelist: parse %q: %w", path, err)
	}
	b.elements = file.Elements
	if b.elements == nil {
		b.elements = []Element{}
	}
	return b, nil
}

// Path returns the JSON file path used for load/save.
func (b *Bundle) Path() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.path
}

// Elements returns a copy of the current whitelist entries.
func (b *Bundle) Elements() []Element {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Element, len(b.elements))
	copy(out, b.elements)
	return out
}

// IsDirty reports whether unsaved changes exist.
func (b *Bundle) IsDirty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dirty
}

// Upsert adds or replaces an element for ip and marks the bundle dirty.
// It does not write to disk. The entry stays active for DefaultPeriod
// after whitelistTime (48h by default).
func (b *Bundle) Upsert(ip string, whitelistTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.elements {
		if b.elements[i].IP == ip {
			b.elements[i].WhitelistTime = whitelistTime
			b.dirty = true
			return
		}
	}
	b.elements = append(b.elements, Element{IP: ip, WhitelistTime: whitelistTime})
	b.dirty = true
}

// UpsertNow whitelists ip from time.Now() for DefaultPeriod and marks dirty.
// It does not write to disk.
func (b *Bundle) UpsertNow(ip string) {
	b.Upsert(ip, time.Now().UTC())
}

// Contains reports whether ip has an active whitelist entry at now
// (WhitelistTime .. WhitelistTime+DefaultPeriod).
func (b *Bundle) Contains(ip string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, el := range b.elements {
		if el.IP == ip && el.Active(now) {
			return true
		}
	}
	return false
}

// Remove deletes the element for ip if present and marks dirty when changed.
// It does not write to disk.
func (b *Bundle) Remove(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.elements {
		if b.elements[i].IP == ip {
			b.elements = append(b.elements[:i], b.elements[i+1:]...)
			b.dirty = true
			return true
		}
	}
	return false
}

// Clear removes all elements and marks dirty when the bundle was non-empty.
// It does not write to disk.
func (b *Bundle) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.elements) == 0 {
		return
	}
	b.elements = nil
	b.dirty = true
}

// MarshalJSON encodes the bundle elements as JSON.
func (b *Bundle) MarshalJSON() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return json.Marshal(bundleFile{Elements: b.elements})
}

// Save writes the bundle to disk when dirty. No-op (and nil error) if clean.
func (b *Bundle) Save() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.saveLocked()
}

func (b *Bundle) saveLocked() error {
	if !b.dirty {
		return nil
	}
	data, err := json.MarshalIndent(bundleFile{Elements: b.elements}, "", "  ")
	if err != nil {
		return fmt.Errorf("ipwhitelist: encode: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(b.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ipwhitelist: mkdir %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(b.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("ipwhitelist: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("ipwhitelist: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("ipwhitelist: close temp: %w", err)
	}
	if err := os.Rename(tmpName, b.path); err != nil {
		return fmt.Errorf("ipwhitelist: rename %q -> %q: %w", tmpName, b.path, err)
	}
	cleanup = false
	b.dirty = false
	return nil
}

// StartPeriodicSave starts a single background loop that saves when dirty at
// the given interval. Returns ErrPeriodicSaveRunning if already active.
func (b *Bundle) StartPeriodicSave(interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("ipwhitelist: interval must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running {
		return ErrPeriodicSaveRunning
	}
	b.stopCh = make(chan struct{})
	b.doneCh = make(chan struct{})
	b.running = true
	stopCh := b.stopCh
	doneCh := b.doneCh
	go b.periodicSaveLoop(interval, stopCh, doneCh)
	return nil
}

// StopPeriodicSave stops the background saver and waits for it to finish.
// It flushes a dirty bundle once before returning.
// Returns ErrPeriodicSaveNotRunning if no saver is active.
func (b *Bundle) StopPeriodicSave() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return ErrPeriodicSaveNotRunning
	}
	stopCh := b.stopCh
	doneCh := b.doneCh
	b.mu.Unlock()

	close(stopCh)
	<-doneCh

	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = false
	b.stopCh = nil
	b.doneCh = nil
	return b.saveLocked()
}

// PeriodicSaveRunning reports whether the periodic saver is active.
func (b *Bundle) PeriodicSaveRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *Bundle) periodicSaveLoop(interval time.Duration, stopCh, doneCh chan struct{}) {
	defer close(doneCh)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			_ = b.Save()
		}
	}
}
