package floodtrack

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/persist"
)

// ErrPeriodicSaveRunning is returned when StartPeriodicSave is called while a
// saver is already active. Only one period is allowed at a time.
var ErrPeriodicSaveRunning = errors.New("floodtrack: periodic save already running")

// ErrPeriodicSaveNotRunning is returned when StopPeriodicSave is called while
// no saver is active.
var ErrPeriodicSaveNotRunning = errors.New("floodtrack: periodic save not running")

// Bundle holds flood tracking entries and persists them as JSON on disk.
// Mutations mark the bundle dirty; disk writes happen on Save or via the
// periodic saver when dirty (not immediately on each append).
// This package only tracks events; it does not enforce bans or punishments.
type Bundle struct {
	mu      sync.Mutex
	path    string
	dirty   bool
	entries []Entry
	saver   persist.Periodic
}

// bundleFile is the on-disk JSON shape.
type bundleFile struct {
	Entries []Entry `json:"entries"`
}

// OpenDefault loads a Bundle from DefaultPath (./data/flood.json).
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
	data, err := persist.ReadFileIfExists(path)
	if err != nil {
		return nil, fmt.Errorf("floodtrack: %w", err)
	}
	if len(data) == 0 {
		return b, nil
	}
	var file bundleFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("floodtrack: parse %q: %w", path, err)
	}
	b.entries = file.Entries
	if b.entries == nil {
		b.entries = []Entry{}
	}
	return b, nil
}

// Path returns the JSON file path used for load/save.
func (b *Bundle) Path() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.path
}

// Entries returns a copy of the current flood tracking entries.
func (b *Bundle) Entries() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Entry, len(b.entries))
	copy(out, b.entries)
	return out
}

// IsDirty reports whether unsaved changes exist.
func (b *Bundle) IsDirty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dirty
}

// Append adds a flood tracking entry and marks the bundle dirty.
// It does not write to disk.
func (b *Bundle) Append(entry Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, entry)
	b.dirty = true
}

// CountSince returns how many entries for ip have Time >= since.
// An empty ip matches nothing.
func (b *Bundle) CountSince(ip string, since time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ip == "" {
		return 0
	}
	n := 0
	for _, e := range b.entries {
		if e.IP == ip && !e.Time.Before(since) {
			n++
		}
	}
	return n
}

// CountSinceService returns how many entries for ip and service have Time >= since.
// An empty ip matches nothing. An empty service matches entries with empty Service.
func (b *Bundle) CountSinceService(ip, service string, since time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ip == "" {
		return 0
	}
	n := 0
	for _, e := range b.entries {
		if e.IP == ip && e.Service == service && !e.Time.Before(since) {
			n++
		}
	}
	return n
}

// RemoveOlderThan deletes entries with Time strictly before cutoff and marks
// dirty when any were removed. It returns the number of removed entries.
// It does not write to disk.
func (b *Bundle) RemoveOlderThan(cutoff time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) == 0 {
		return 0
	}
	kept := b.entries[:0]
	removed := 0
	for _, e := range b.entries {
		if e.Time.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	if removed == 0 {
		return 0
	}
	if len(kept) == 0 {
		b.entries = nil
	} else {
		b.entries = kept
	}
	b.dirty = true
	return removed
}

// RemoveForIP deletes all entries for ip and marks dirty when any were removed.
func (b *Bundle) RemoveForIP(ip string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ip == "" || len(b.entries) == 0 {
		return 0
	}
	kept := b.entries[:0]
	removed := 0
	for _, e := range b.entries {
		if e.IP == ip {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	if removed == 0 {
		return 0
	}
	if len(kept) == 0 {
		b.entries = nil
	} else {
		b.entries = kept
	}
	b.dirty = true
	return removed
}

// Clear removes all entries and marks dirty when the bundle was non-empty.
// It does not write to disk.
func (b *Bundle) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) == 0 {
		return
	}
	b.entries = nil
	b.dirty = true
}

// MarshalJSON encodes the bundle entries as JSON (compact).
func (b *Bundle) MarshalJSON() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return encodeBundle(b.entries, false)
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
	data, err := encodeBundle(b.entries, true)
	if err != nil {
		return fmt.Errorf("floodtrack: encode: %w", err)
	}
	if err := persist.WriteAtomic(b.path, data); err != nil {
		return fmt.Errorf("floodtrack: %w", err)
	}
	b.dirty = false
	return nil
}

func encodeBundle(entries []Entry, indent bool) ([]byte, error) {
	file := bundleFile{Entries: entries}
	if entries == nil {
		file.Entries = []Entry{}
	}
	var (
		data []byte
		err  error
	)
	if indent {
		data, err = json.MarshalIndent(file, "", "  ")
	} else {
		data, err = json.Marshal(file)
	}
	if err != nil {
		return nil, err
	}
	if indent {
		data = append(data, '\n')
	}
	return data, nil
}

// StartPeriodicSave starts a single background loop that saves when dirty at
// the given interval. Non-positive interval uses DefaultSaveInterval.
// Returns ErrPeriodicSaveRunning if already active.
func (b *Bundle) StartPeriodicSave(interval time.Duration) error {
	err := b.saver.Start(interval, DefaultSaveInterval, b.Save, "floodtrack")
	if errors.Is(err, persist.ErrAlreadyRunning) {
		return ErrPeriodicSaveRunning
	}
	return err
}

// StopPeriodicSave stops the background saver and waits for it to finish.
// It flushes a dirty bundle once before returning.
// Returns ErrPeriodicSaveNotRunning if no saver is active.
func (b *Bundle) StopPeriodicSave() error {
	err := b.saver.Stop(b.Save)
	if errors.Is(err, persist.ErrNotRunning) {
		return ErrPeriodicSaveNotRunning
	}
	return err
}

// PeriodicSaveRunning reports whether the periodic saver is active.
func (b *Bundle) PeriodicSaveRunning() bool {
	return b.saver.Running()
}
