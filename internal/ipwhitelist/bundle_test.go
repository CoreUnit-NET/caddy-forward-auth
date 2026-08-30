package ipwhitelist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if b.IsDirty() {
		t.Fatal("expected clean empty bundle")
	}
	if got := b.Elements(); len(got) != 0 {
		t.Fatalf("elements=%v, want empty", got)
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
	if DefaultPath != "./data/ipwhitelist.json" {
		t.Fatalf("DefaultPath=%q, want ./data/ipwhitelist.json", DefaultPath)
	}
	if DefaultPeriod != 48*time.Hour {
		t.Fatalf("DefaultPeriod=%v, want 48h", DefaultPeriod)
	}
}

func TestElementActiveDefaultPeriod(t *testing.T) {
	start := time.Now().UTC().Truncate(time.Second)
	el := Element{IP: "1.2.3.4", WhitelistTime: start}

	if !el.ExpiresAt(DefaultPeriod).Equal(start.Add(48 * time.Hour)) {
		t.Fatalf("ExpiresAt=%v", el.ExpiresAt(DefaultPeriod))
	}
	if !el.Active(start, DefaultPeriod) {
		t.Fatal("expected active at start")
	}
	if !el.Active(start.Add(47*time.Hour), DefaultPeriod) {
		t.Fatal("expected active before 48h")
	}
	if el.Active(start.Add(48*time.Hour), DefaultPeriod) {
		t.Fatal("expected inactive at exact expiry")
	}
	if el.Active(start.Add(-time.Second), DefaultPeriod) {
		t.Fatal("expected inactive before whitelist time")
	}
}

func TestContainsRespectsDefaultPeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	start := time.Now().UTC().Truncate(time.Second)
	b.Upsert("10.0.0.2", start)

	if !b.Contains("10.0.0.2", start.Add(time.Hour)) {
		t.Fatal("expected contains while active")
	}
	if b.Contains("10.0.0.2", start.Add(DefaultPeriod)) {
		t.Fatal("expected not contains after DefaultPeriod")
	}
	if b.Contains("10.0.0.9", start.Add(time.Hour)) {
		t.Fatal("expected missing ip not contained")
	}
}

func TestUpsertRemoveClearDirtyWithoutImmediateSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ts := time.Now().UTC().Truncate(time.Second)
	b.Upsert("1.2.3.4", ts)
	if !b.IsDirty() {
		t.Fatal("expected dirty after Upsert")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no file write on Upsert, stat err=%v", err)
	}

	elems := b.Elements()
	if len(elems) != 1 || elems[0].IP != "1.2.3.4" || !elems[0].WhitelistTime.Equal(ts) {
		t.Fatalf("unexpected elements: %#v", elems)
	}

	b.Upsert("1.2.3.4", ts) // already dirty; identical timestamp is a no-op for content
	if !b.IsDirty() {
		t.Fatal("expected still dirty after identical Upsert")
	}
	b.Upsert("1.2.3.4", ts.Add(time.Minute))
	if len(b.Elements()) != 1 {
		t.Fatalf("expected replace, got %#v", b.Elements())
	}

	if !b.Remove("1.2.3.4") {
		t.Fatal("expected Remove true")
	}
	if b.Remove("1.2.3.4") {
		t.Fatal("expected Remove false for missing ip")
	}
	if !b.IsDirty() {
		t.Fatal("expected dirty after Remove")
	}

	b.Upsert("9.9.9.9", ts)
	b.Clear()
	if len(b.Elements()) != 0 {
		t.Fatal("expected empty after Clear")
	}
	if !b.IsDirty() {
		t.Fatal("expected dirty after Clear")
	}
}

func TestUpsertSkipsDirtyWhenUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ts := time.Now().UTC().Truncate(time.Second)
	b.Upsert("1.2.3.4", ts)
	if err := b.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if b.IsDirty() {
		t.Fatal("expected clean after Save")
	}
	b.Upsert("1.2.3.4", ts)
	if b.IsDirty() {
		t.Fatal("identical Upsert must not mark dirty")
	}
}

func TestSaveOnlyWhenDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
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

	ts := time.Now().UTC().Truncate(time.Second)
	b.Upsert("10.0.0.1", ts)
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
	if len(file.Elements) != 1 || file.Elements[0].IP != "10.0.0.1" {
		t.Fatalf("file contents: %#v", file)
	}

	if err := b.Save(); err != nil {
		t.Fatalf("second clean Save: %v", err)
	}
}

func TestSavePrunesExpiredKeepsFuture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	b.Upsert("10.0.0.1", now.Add(-DefaultPeriod-time.Hour)) // expired
	b.Upsert("10.0.0.2", now)                               // active
	b.Upsert("10.0.0.3", now.Add(time.Hour))                // not yet started
	if err := b.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	elems := b.Elements()
	if len(elems) != 2 {
		t.Fatalf("want 2 kept elements, got %#v", elems)
	}
	ips := map[string]bool{}
	for _, el := range elems {
		ips[el.IP] = true
	}
	if ips["10.0.0.1"] || !ips["10.0.0.2"] || !ips["10.0.0.3"] {
		t.Fatalf("unexpected kept ips: %#v", elems)
	}
}

func TestOpenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ts := time.Now().UTC().Truncate(time.Second)
	b.Upsert("2001:db8::1", ts)
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
	elems := loaded.Elements()
	if len(elems) != 1 || elems[0].IP != "2001:db8::1" || !elems[0].WhitelistTime.Equal(ts) {
		t.Fatalf("loaded elements: %#v", elems)
	}
}

func TestMarshalJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ts := time.Now().UTC().Truncate(time.Second)
	b.Upsert("127.0.0.1", ts)

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var file bundleFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(file.Elements) != 1 || file.Elements[0].IP != "127.0.0.1" {
		t.Fatalf("got %#v", file)
	}
}

func TestPeriodicSaveDirtyOnlyAndSinglePeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
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

	// No dirty changes: file must stay absent across a tick.
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no save while clean, stat err=%v", err)
	}

	ts := time.Now().UTC().Truncate(time.Second)
	b.Upsert("8.8.8.8", ts)

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
	elems := loaded.Elements()
	if len(elems) != 1 || elems[0].IP != "8.8.8.8" {
		t.Fatalf("loaded after periodic: %#v", elems)
	}
}

func TestStartPeriodicSaveNonPositiveUsesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitelist.json")
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
	path := filepath.Join(t.TempDir(), "whitelist.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Long interval so the ticker will not save before Stop.
	if err := b.StartPeriodicSave(time.Hour); err != nil {
		t.Fatalf("Start: %v", err)
	}
	b.Upsert("1.1.1.1", time.Now().UTC().Truncate(time.Second))
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
	if len(loaded.Elements()) != 1 {
		t.Fatalf("want 1 element after flush, got %#v", loaded.Elements())
	}
}
