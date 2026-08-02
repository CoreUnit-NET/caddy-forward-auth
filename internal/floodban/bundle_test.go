package floodban

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "ban.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if b.IsDirty() {
		t.Fatal("expected clean empty bundle")
	}
	if got := b.Bans(); len(got) != 0 {
		t.Fatalf("bans=%v, want empty", got)
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
	if DefaultPath != "./data/ban.json" {
		t.Fatalf("DefaultPath=%q, want ./data/ban.json", DefaultPath)
	}
}

func TestBanActive(t *testing.T) {
	start := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	temp := Ban{
		IP:        "1.2.3.4",
		BannedAt:  start,
		ExpiresAt: start.Add(3 * time.Minute),
	}
	if !temp.Active(start) {
		t.Fatal("expected temp active at start")
	}
	if !temp.Active(start.Add(2 * time.Minute)) {
		t.Fatal("expected temp active before expiry")
	}
	if temp.Active(start.Add(3 * time.Minute)) {
		t.Fatal("expected temp inactive at expiry")
	}

	perma := Ban{IP: "1.2.3.4", Permanent: true, BannedAt: start}
	if !perma.Active(start.Add(10000 * time.Hour)) {
		t.Fatal("expected permanent always active")
	}
	if (Ban{}).Active(start) {
		t.Fatal("empty ban should be inactive")
	}
}

func TestUpsertEscalation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	start := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)

	b.Upsert(Ban{
		IP:        "10.0.0.1",
		BannedAt:  start,
		ExpiresAt: start.Add(3 * time.Minute),
		Rule:      "10/2m",
	})
	if !b.IsDirty() {
		t.Fatal("expected dirty after Upsert")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no file write on Upsert, stat err=%v", err)
	}

	// Weaker temp ban must not replace.
	b.Upsert(Ban{
		IP:        "10.0.0.1",
		BannedAt:  start,
		ExpiresAt: start.Add(time.Minute),
		Rule:      "weaker",
	})
	got, ok := b.Get("10.0.0.1")
	if !ok || got.Rule != "10/2m" {
		t.Fatalf("weaker upsert changed ban: %#v", got)
	}

	// Stronger temp ban replaces.
	b.Upsert(Ban{
		IP:        "10.0.0.1",
		BannedAt:  start,
		ExpiresAt: start.Add(2 * time.Hour),
		Rule:      "60/30m",
	})
	got, _ = b.Get("10.0.0.1")
	if got.Rule != "60/30m" || !got.ExpiresAt.Equal(start.Add(2*time.Hour)) {
		t.Fatalf("expected stronger temp ban, got %#v", got)
	}

	// Permanent replaces temp.
	b.Upsert(Ban{
		IP:        "10.0.0.1",
		Permanent: true,
		BannedAt:  start,
		Rule:      "90/60m",
	})
	got, _ = b.Get("10.0.0.1")
	if !got.Permanent || got.Rule != "90/60m" {
		t.Fatalf("expected permanent ban, got %#v", got)
	}

	// Temp must not downgrade permanent.
	b.Upsert(Ban{
		IP:        "10.0.0.1",
		BannedAt:  start,
		ExpiresAt: start.Add(24 * time.Hour),
		Rule:      "temp",
	})
	got, _ = b.Get("10.0.0.1")
	if !got.Permanent || got.Rule != "90/60m" {
		t.Fatalf("permanent downgraded: %#v", got)
	}

	banned, ok := b.IsBanned("10.0.0.1", start.Add(time.Hour))
	if !ok || !banned.Permanent {
		t.Fatalf("IsBanned=%v %#v", ok, banned)
	}
	if _, ok := b.IsBanned("9.9.9.9", start); ok {
		t.Fatal("expected missing ip not banned")
	}
}

func TestRemoveAndRemoveExpired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	start := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	b.Upsert(Ban{IP: "1.1.1.1", BannedAt: start, ExpiresAt: start.Add(time.Minute), Rule: "temp"})
	b.Upsert(Ban{IP: "2.2.2.2", Permanent: true, BannedAt: start, Rule: "perma"})
	b.Upsert(Ban{IP: "3.3.3.3", BannedAt: start, ExpiresAt: start.Add(time.Hour), Rule: "active"})

	removed := b.RemoveExpired(start.Add(2 * time.Minute))
	if removed != 1 {
		t.Fatalf("RemoveExpired=%d, want 1", removed)
	}
	if _, ok := b.Get("1.1.1.1"); ok {
		t.Fatal("expired temp ban should be gone")
	}
	if _, ok := b.Get("2.2.2.2"); !ok {
		t.Fatal("permanent ban should remain")
	}
	if _, ok := b.Get("3.3.3.3"); !ok {
		t.Fatal("active temp ban should remain")
	}

	if !b.Remove("2.2.2.2") {
		t.Fatal("expected Remove true")
	}
	if b.Remove("2.2.2.2") {
		t.Fatal("expected Remove false")
	}

	b.Clear()
	if len(b.Bans()) != 0 {
		t.Fatal("expected empty after Clear")
	}
}

func TestSaveOnlyWhenDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
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

	start := time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC)
	b.Upsert(Ban{IP: "10.0.0.1", Permanent: true, BannedAt: start, Rule: "perma"})
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
	if len(file.Bans) != 1 || file.Bans[0].IP != "10.0.0.1" || !file.Bans[0].Permanent {
		t.Fatalf("file contents: %#v", file)
	}
}

func TestOpenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	start := time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC)
	b.Upsert(Ban{
		IP:        "2001:db8::1",
		BannedAt:  start,
		ExpiresAt: start.Add(2 * time.Hour),
		Rule:      "60/30m",
	})
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
	bans := loaded.Bans()
	if len(bans) != 1 || bans[0].IP != "2001:db8::1" || bans[0].Rule != "60/30m" {
		t.Fatalf("loaded bans: %#v", bans)
	}
}

func TestMarshalJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b.Upsert(Ban{IP: "127.0.0.1", Permanent: true, BannedAt: start})

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var file bundleFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(file.Bans) != 1 || file.Bans[0].IP != "127.0.0.1" {
		t.Fatalf("got %#v", file)
	}
}

func TestPeriodicSaveDirtyOnlyAndSinglePeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
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

	start := time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC)
	b.Upsert(Ban{IP: "8.8.8.8", Permanent: true, BannedAt: start, Rule: "perma"})

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
	bans := loaded.Bans()
	if len(bans) != 1 || bans[0].IP != "8.8.8.8" {
		t.Fatalf("loaded after periodic: %#v", bans)
	}
}

func TestStopPeriodicSaveFlushesDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ban.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.StartPeriodicSave(time.Hour); err != nil {
		t.Fatalf("Start: %v", err)
	}
	start := time.Date(2026, 8, 2, 16, 0, 0, 0, time.UTC)
	b.Upsert(Ban{IP: "1.1.1.1", Permanent: true, BannedAt: start})
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
	if len(loaded.Bans()) != 1 {
		t.Fatalf("want 1 ban after flush, got %#v", loaded.Bans())
	}
}
