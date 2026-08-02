package floodtrack

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if b.IsDirty() {
		t.Fatal("expected clean empty bundle")
	}
	if got := b.Entries(); len(got) != 0 {
		t.Fatalf("entries=%v, want empty", got)
	}
	if b.Path() != path {
		t.Fatalf("Path=%q, want %q", b.Path(), path)
	}
}

func TestOpenEmptyPathUsesDefault(t *testing.T) {
	b, err := Open("")
	if err != nil {
		t.Fatalf("Open(\"\"): %v", err)
	}
	if b.Path() != DefaultPath {
		t.Fatalf("Path=%q, want %q", b.Path(), DefaultPath)
	}
}

func TestOpenDefault(t *testing.T) {
	b, err := OpenDefault()
	if err != nil {
		t.Fatalf("OpenDefault: %v", err)
	}
	if b.Path() != DefaultPath {
		t.Fatalf("Path=%q, want %q", b.Path(), DefaultPath)
	}
	if DefaultPath != "./data/flood.json" {
		t.Fatalf("DefaultPath=%q, want ./data/flood.json", DefaultPath)
	}
	if DefaultRetention != 168*time.Hour {
		t.Fatalf("DefaultRetention=%v, want 168h", DefaultRetention)
	}
}

func TestAppendCountAndRemoveOlderThan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	base := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	b.Append(Entry{Time: base.Add(-2 * time.Hour), IP: "1.2.3.4", Service: "test"})
	b.Append(Entry{Time: base.Add(-time.Hour), IP: "1.2.3.4", Service: "intern"})
	b.Append(Entry{Time: base, IP: "1.2.3.4", Service: "test"})
	b.Append(Entry{Time: base, IP: "9.9.9.9", Service: "test"})

	if !b.IsDirty() {
		t.Fatal("expected dirty after Append")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no file write on Append, stat err=%v", err)
	}

	if got := b.CountSince("1.2.3.4", base.Add(-90*time.Minute)); got != 2 {
		t.Fatalf("CountSince last 90m = %d, want 2", got)
	}
	if got := b.CountSince("1.2.3.4", base.Add(-3*time.Hour)); got != 3 {
		t.Fatalf("CountSince last 3h = %d, want 3", got)
	}
	if got := b.CountSinceService("1.2.3.4", "test", base.Add(-3*time.Hour)); got != 2 {
		t.Fatalf("CountSinceService test = %d, want 2", got)
	}
	if got := b.CountSince("", base); got != 0 {
		t.Fatalf("empty ip CountSince = %d, want 0", got)
	}

	removed := b.RemoveOlderThan(base.Add(-90 * time.Minute))
	if removed != 1 {
		t.Fatalf("RemoveOlderThan removed=%d, want 1", removed)
	}
	if len(b.Entries()) != 3 {
		t.Fatalf("entries after prune=%d, want 3", len(b.Entries()))
	}

	b.Clear()
	if len(b.Entries()) != 0 {
		t.Fatal("expected empty after Clear")
	}
	if !b.IsDirty() {
		t.Fatal("expected dirty after Clear")
	}
}

func TestSaveOnlyWhenDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.Save(); err != nil {
		t.Fatalf("Save clean: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("clean Save must not create file")
	}

	ts := time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC)
	b.Append(Entry{Time: ts, IP: "10.0.0.1", Service: "test"})
	if err := b.Save(); err != nil {
		t.Fatalf("Save dirty: %v", err)
	}
	if b.IsDirty() {
		t.Fatal("expected clean after Save")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var file bundleFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(file.Entries) != 1 || file.Entries[0].IP != "10.0.0.1" || file.Entries[0].Service != "test" {
		t.Fatalf("file contents: %#v", file)
	}

	if err := b.Save(); err != nil {
		t.Fatalf("second clean Save: %v", err)
	}
}

func TestOpenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ts := time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC)
	b.Append(Entry{Time: ts, IP: "2001:db8::1", Service: "intern"})
	if err := b.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open again: %v", err)
	}
	if loaded.IsDirty() {
		t.Fatal("loaded bundle should be clean")
	}
	entries := loaded.Entries()
	if len(entries) != 1 || entries[0].IP != "2001:db8::1" || entries[0].Service != "intern" || !entries[0].Time.Equal(ts) {
		t.Fatalf("loaded entries: %#v", entries)
	}
}

func TestMarshalJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b.Append(Entry{Time: ts, IP: "127.0.0.1", Service: ""})

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var file bundleFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(file.Entries) != 1 || file.Entries[0].IP != "127.0.0.1" {
		t.Fatalf("got %#v", file)
	}
}

func TestPeriodicSaveDirtyOnlyAndSinglePeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if b.PeriodicSaveRunning() {
		t.Fatal("expected not running initially")
	}
	if err := b.StopPeriodicSave(); !errors.Is(err, ErrPeriodicSaveNotRunning) {
		t.Fatalf("Stop: got %v, want ErrPeriodicSaveNotRunning", err)
	}

	if err := b.StartPeriodicSave(20 * time.Millisecond); err != nil {
		t.Fatalf("StartPeriodicSave: %v", err)
	}
	if !b.PeriodicSaveRunning() {
		t.Fatal("expected running")
	}
	if err := b.StartPeriodicSave(20 * time.Millisecond); !errors.Is(err, ErrPeriodicSaveRunning) {
		t.Fatalf("second Start: got %v, want ErrPeriodicSaveRunning", err)
	}

	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no save while clean, stat err=%v", err)
	}

	b.Append(Entry{
		Time:    time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC),
		IP:      "8.8.8.8",
		Service: "test",
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for periodic dirty save")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if b.IsDirty() {
		t.Fatal("expected clean after periodic save")
	}

	if err := b.StopPeriodicSave(); err != nil {
		t.Fatalf("StopPeriodicSave: %v", err)
	}
	if b.PeriodicSaveRunning() {
		t.Fatal("expected not running after stop")
	}

	loaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open after periodic save: %v", err)
	}
	entries := loaded.Entries()
	if len(entries) != 1 || entries[0].IP != "8.8.8.8" {
		t.Fatalf("loaded after periodic: %#v", entries)
	}
}

func TestStartPeriodicSaveNonPositiveUsesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.StartPeriodicSave(0); err != nil {
		t.Fatalf("StartPeriodicSave(0): %v", err)
	}
	if !b.PeriodicSaveRunning() {
		t.Fatal("expected running with default interval")
	}
	if err := b.StopPeriodicSave(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestStopPeriodicSaveFlushesDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flood.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.StartPeriodicSave(time.Hour); err != nil {
		t.Fatalf("Start: %v", err)
	}
	b.Append(Entry{
		Time:    time.Date(2026, 8, 2, 16, 0, 0, 0, time.UTC),
		IP:      "1.1.1.1",
		Service: "test",
	})
	if err := b.StopPeriodicSave(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if b.IsDirty() {
		t.Fatal("expected flush on stop")
	}
	loaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(loaded.Entries()) != 1 {
		t.Fatalf("want 1 entry after flush, got %#v", loaded.Entries())
	}
}
