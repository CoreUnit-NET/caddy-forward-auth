package ipwhitelist

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
	saver    persist.Periodic
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
	data, err := persist.ReadFileIfExists(path)
	if err != nil {
		return nil, fmt.Errorf("ipwhitelist: %w", err)
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

// Upsert adds or replaces an element for ip and marks the bundle dirty when
// the stored value changes. It does not write to disk. The entry stays active
// for DefaultPeriod after whitelistTime (48h by default).
func (b *Bundle) Upsert(ip string, whitelistTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.elements {
		if b.elements[i].IP == ip {
			if b.elements[i].WhitelistTime.Equal(whitelistTime) {
				return
			}
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

// MarshalJSON encodes the bundle elements as JSON (compact).
func (b *Bundle) MarshalJSON() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return encodeBundle(b.elements, false)
}

// Save writes the bundle to disk when dirty. No-op (and nil error) if clean.
// Expired entries are pruned before writing.
func (b *Bundle) Save() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.saveLocked(time.Now().UTC())
}

func (b *Bundle) saveLocked(now time.Time) error {
	if b.pruneExpiredLocked(now) {
		b.dirty = true
	}
	if !b.dirty {
		return nil
	}
	data, err := encodeBundle(b.elements, true)
	if err != nil {
		return fmt.Errorf("ipwhitelist: encode: %w", err)
	}
	if err := persist.WriteAtomic(b.path, data); err != nil {
		return fmt.Errorf("ipwhitelist: %w", err)
	}
	b.dirty = false
	return nil
}

func (b *Bundle) pruneExpiredLocked(now time.Time) bool {
	if len(b.elements) == 0 {
		return false
	}
	kept := b.elements[:0]
	removed := false
	for _, el := range b.elements {
		// Drop empty/invalid and entries whose whitelist window has ended.
		// Keep not-yet-started entries (WhitelistTime in the future).
		if el.IP == "" || el.WhitelistTime.IsZero() || !now.Before(el.ExpiresAt()) {
			removed = true
			continue
		}
		kept = append(kept, el)
	}
	if !removed {
		return false
	}
	if len(kept) == 0 {
		b.elements = nil
	} else {
		b.elements = kept
	}
	return true
}

func encodeBundle(elements []Element, indent bool) ([]byte, error) {
	file := bundleFile{Elements: elements}
	if elements == nil {
		file.Elements = []Element{}
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
	err := b.saver.Start(interval, DefaultSaveInterval, b.Save, "ipwhitelist")
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
