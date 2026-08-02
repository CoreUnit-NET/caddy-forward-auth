package floodban

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
var ErrPeriodicSaveRunning = errors.New("floodban: periodic save already running")

// ErrPeriodicSaveNotRunning is returned when StopPeriodicSave is called while
// no saver is active.
var ErrPeriodicSaveNotRunning = errors.New("floodban: periodic save not running")

// Bundle holds ban entries and persists them as JSON on disk.
// Mutations mark the bundle dirty; disk writes happen on Save or via the
// periodic saver when dirty (not immediately on each change).
// This package only stores bans; it does not enforce them over HTTP.
type Bundle struct {
	mu    sync.Mutex
	path  string
	dirty bool
	bans  []Ban
	saver persist.Periodic
}

// bundleFile is the on-disk JSON shape.
type bundleFile struct {
	Bans []Ban `json:"bans"`
}

// OpenDefault loads a Bundle from DefaultPath (./data/ban.json).
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
		return nil, fmt.Errorf("floodban: %w", err)
	}
	if len(data) == 0 {
		return b, nil
	}
	var file bundleFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("floodban: parse %q: %w", path, err)
	}
	b.bans = file.Bans
	if b.bans == nil {
		b.bans = []Ban{}
	}
	return b, nil
}

// Path returns the JSON file path used for load/save.
func (b *Bundle) Path() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.path
}

// Bans returns a copy of the current ban entries.
func (b *Bundle) Bans() []Ban {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Ban, len(b.bans))
	copy(out, b.bans)
	return out
}

// IsDirty reports whether unsaved changes exist.
func (b *Bundle) IsDirty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dirty
}

// Upsert adds or upgrades a ban for ban.IP.
// Existing permanent bans are never downgraded. A temp ban only replaces
// another temp ban when the new ExpiresAt is later. Unchanged state is not
// marked dirty. It does not write to disk.
func (b *Bundle) Upsert(ban Ban) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ban.IP == "" {
		return
	}
	for i := range b.bans {
		if b.bans[i].IP != ban.IP {
			continue
		}
		cur := b.bans[i]
		if cur.Permanent {
			// Keep permanent; optionally refresh rule metadata only when identical.
			return
		}
		if !ban.HarsherThan(cur) && ban.Permanent == cur.Permanent && ban.ExpiresAt.Equal(cur.ExpiresAt) && ban.Rule == cur.Rule && ban.BannedAt.Equal(cur.BannedAt) {
			return
		}
		if !ban.HarsherThan(cur) && !ban.Permanent {
			// Weaker or equal temp ban: ignore.
			return
		}
		b.bans[i] = ban
		b.dirty = true
		return
	}
	b.bans = append(b.bans, ban)
	b.dirty = true
}

// Get returns the ban for ip if present.
func (b *Bundle) Get(ip string) (Ban, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ban := range b.bans {
		if ban.IP == ip {
			return ban, true
		}
	}
	return Ban{}, false
}

// IsBanned reports whether ip has an active ban at now.
// Permanent bans are preferred when both somehow exist (Upsert keeps one per IP).
func (b *Bundle) IsBanned(ip string, now time.Time) (Ban, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ban := range b.bans {
		if ban.IP == ip && ban.Active(now) {
			return ban, true
		}
	}
	return Ban{}, false
}

// Remove deletes the ban for ip if present and marks dirty when changed.
// It does not write to disk.
func (b *Bundle) Remove(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.bans {
		if b.bans[i].IP == ip {
			b.bans = append(b.bans[:i], b.bans[i+1:]...)
			b.dirty = true
			return true
		}
	}
	return false
}

// RemoveExpired deletes inactive temporary bans at now and marks dirty when
// any were removed. Permanent bans are kept. Returns the number removed.
// It does not write to disk.
func (b *Bundle) RemoveExpired(now time.Time) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.bans) == 0 {
		return 0
	}
	kept := b.bans[:0]
	removed := 0
	for _, ban := range b.bans {
		if ban.Permanent || ban.Active(now) {
			kept = append(kept, ban)
			continue
		}
		removed++
	}
	if removed == 0 {
		return 0
	}
	if len(kept) == 0 {
		b.bans = nil
	} else {
		b.bans = kept
	}
	b.dirty = true
	return removed
}

// Clear removes all bans and marks dirty when the bundle was non-empty.
// It does not write to disk.
func (b *Bundle) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.bans) == 0 {
		return
	}
	b.bans = nil
	b.dirty = true
}

// MarshalJSON encodes the bundle bans as JSON (compact).
func (b *Bundle) MarshalJSON() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return encodeBundle(b.bans, false)
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
	data, err := encodeBundle(b.bans, true)
	if err != nil {
		return fmt.Errorf("floodban: encode: %w", err)
	}
	if err := persist.WriteAtomic(b.path, data); err != nil {
		return fmt.Errorf("floodban: %w", err)
	}
	b.dirty = false
	return nil
}

func encodeBundle(bans []Ban, indent bool) ([]byte, error) {
	file := bundleFile{Bans: bans}
	if bans == nil {
		file.Bans = []Ban{}
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
	err := b.saver.Start(interval, DefaultSaveInterval, b.Save, "floodban")
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
