package persist

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriteAtomicAndReadFileIfExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if data, err := ReadFileIfExists(path); err != nil || data != nil {
		t.Fatalf("missing file: data=%v err=%v", data, err)
	}
	payload := []byte("{\"ok\":true}\n")
	if err := WriteAtomic(path, payload); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	got, err := ReadFileIfExists(path)
	if err != nil {
		t.Fatalf("ReadFileIfExists: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestPeriodicStartStop(t *testing.T) {
	var p Periodic
	var calls atomic.Int32
	save := func() error {
		calls.Add(1)
		return nil
	}

	if err := p.Stop(save); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop: %v", err)
	}
	if err := p.Start(15*time.Millisecond, time.Second, save, "test"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !p.Running() {
		t.Fatal("expected running")
	}
	if err := p.Start(15*time.Millisecond, time.Second, save, "test"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Start: %v", err)
	}

	deadline := time.Now().Add(300 * time.Millisecond)
	for calls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for periodic save")
		}
		time.Sleep(5 * time.Millisecond)
	}

	path := filepath.Join(t.TempDir(), "flush.json")
	if err := p.Stop(func() error {
		return WriteAtomic(path, []byte("flushed"))
	}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if p.Running() {
		t.Fatal("expected stopped")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("flush file: %v", err)
	}
}
