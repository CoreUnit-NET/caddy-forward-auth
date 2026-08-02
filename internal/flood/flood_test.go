package flood

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/floodban"
	"github.com/CoreUnit-NET/intern-auth-gateway/internal/floodtrack"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	eng, err := Open(filepath.Join(dir, "flood.json"), filepath.Join(dir, "ban.json"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return eng
}

func TestRecordFailureAppliesTempBan(t *testing.T) {
	eng := testEngine(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	ip := "1.2.3.4"

	for i := 0; i < 9; i++ {
		eng.RecordFailure(ip, "test", now.Add(time.Duration(i)*time.Second))
	}
	if _, ok := eng.Bans.IsBanned(ip, now.Add(10*time.Second)); ok {
		t.Fatal("expected no ban before 10 failures")
	}

	eng.RecordFailure(ip, "test", now.Add(10*time.Second))
	ban, ok := eng.Bans.IsBanned(ip, now.Add(10*time.Second))
	if !ok {
		t.Fatal("expected temp ban after 10 failures in 2m")
	}
	if ban.Permanent || ban.Rule != "10/2m" {
		t.Fatalf("ban=%#v, want temp 10/2m", ban)
	}
	if !ban.ExpiresAt.Equal(now.Add(10 * time.Second).Add(3 * time.Minute)) {
		t.Fatalf("ExpiresAt=%v", ban.ExpiresAt)
	}
}

func TestRecordFailureEscalatesToPermanent(t *testing.T) {
	eng := testEngine(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	ip := "5.5.5.5"

	// 90 failures within 60 minutes => permanent.
	for i := 0; i < 90; i++ {
		eng.RecordFailure(ip, "intern", now.Add(time.Duration(i)*time.Second))
	}
	ban, ok := eng.Bans.IsBanned(ip, now.Add(90*time.Second))
	if !ok || !ban.Permanent {
		t.Fatalf("expected permanent ban, got ok=%v %#v", ok, ban)
	}
	if ban.Rule != "90/60m" {
		t.Fatalf("Rule=%q, want 90/60m", ban.Rule)
	}
}

func TestHarshestRuleWins(t *testing.T) {
	eng := testEngine(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	ip := "8.8.8.8"

	// Seed 60 failures in the last 30 minutes without going through RecordFailure
	// so we control the exact moment UpdatePunishments runs.
	for i := 0; i < 60; i++ {
		eng.Track.Append(floodtrack.Entry{
			Time:    now.Add(-29 * time.Minute).Add(time.Duration(i) * time.Second),
			IP:      ip,
			Service: "test",
		})
	}
	eng.UpdatePunishments(ip, now)
	ban, ok := eng.Bans.IsBanned(ip, now)
	if !ok || ban.Rule != "60/30m" {
		t.Fatalf("expected 60/30m ban, got ok=%v %#v", ok, ban)
	}
	if ban.Permanent || !ban.ExpiresAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("expected 2h temp ban, got %#v", ban)
	}
}

func TestCheckBanPermanentNoRecord(t *testing.T) {
	eng := testEngine(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	ip := "9.9.9.9"
	eng.Bans.Upsert(floodban.Ban{IP: ip, Permanent: true, BannedAt: now, Rule: "manual"})

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-For", ip)
	rr := httptest.NewRecorder()

	ban, blocked := eng.CheckBan(rr, req, "test", now)
	if !blocked || !ban.Permanent {
		t.Fatalf("blocked=%v ban=%#v", blocked, ban)
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rr.Code)
	}
	if got := len(eng.Track.Entries()); got != 0 {
		t.Fatalf("permanent ban must not record flood events, got %d", got)
	}
}

func TestCheckBanTempStillRecords(t *testing.T) {
	eng := testEngine(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	ip := "7.7.7.7"
	eng.Bans.Upsert(floodban.Ban{
		IP:        ip,
		BannedAt:  now,
		ExpiresAt: now.Add(3 * time.Minute),
		Rule:      "10/2m",
	})

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.RemoteAddr = ip + ":12345"
	rr := httptest.NewRecorder()

	_, blocked := eng.CheckBan(rr, req, "test", now.Add(time.Second))
	if !blocked {
		t.Fatal("expected temp ban blocked")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rr.Code)
	}
	if got := len(eng.Track.Entries()); got != 1 {
		t.Fatalf("temp ban should record failure, got %d entries", got)
	}
}

func TestClientIPPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	req.Header.Set("X-Real-IP", "10.0.0.2")
	req.Header.Set("X-Forwarded-For", "10.0.0.3, 10.0.0.4")
	if got := ClientIP(req); got != "10.0.0.3" {
		t.Fatalf("ClientIP=%q, want 10.0.0.3", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:9999"
	req2.Header.Set("X-Real-IP", "10.0.0.2")
	if got := ClientIP(req2); got != "10.0.0.2" {
		t.Fatalf("ClientIP=%q, want 10.0.0.2", got)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "10.0.0.1:9999"
	if got := ClientIP(req3); got != "10.0.0.1" {
		t.Fatalf("ClientIP=%q, want 10.0.0.1", got)
	}
}

func TestCleanupRemovesOldEntriesAndExpiredBans(t *testing.T) {
	eng := testEngine(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)

	eng.Track.Append(floodtrack.Entry{
		Time: now.Add(-floodtrack.DefaultRetention - time.Hour),
		IP:   "1.1.1.1",
	})
	eng.Track.Append(floodtrack.Entry{
		Time: now.Add(-time.Hour),
		IP:   "1.1.1.1",
	})
	eng.Bans.Upsert(floodban.Ban{
		IP:        "2.2.2.2",
		BannedAt:  now.Add(-time.Hour),
		ExpiresAt: now.Add(-time.Minute),
		Rule:      "expired",
	})
	eng.Bans.Upsert(floodban.Ban{
		IP:        "3.3.3.3",
		Permanent: true,
		BannedAt:  now.Add(-200 * time.Hour),
		Rule:      "perma",
	})

	eng.Cleanup(now)

	if got := len(eng.Track.Entries()); got != 1 {
		t.Fatalf("track entries=%d, want 1", got)
	}
	if _, ok := eng.Bans.Get("2.2.2.2"); ok {
		t.Fatal("expired temp ban should be removed")
	}
	if _, ok := eng.Bans.Get("3.3.3.3"); !ok {
		t.Fatal("permanent ban should remain")
	}
}

func TestStartStopCleanup(t *testing.T) {
	eng := testEngine(t)
	if eng.CleanupRunning() {
		t.Fatal("expected not running")
	}
	if err := eng.StartCleanup(50 * time.Millisecond); err != nil {
		t.Fatalf("StartCleanup: %v", err)
	}
	if !eng.CleanupRunning() {
		t.Fatal("expected running")
	}
	if err := eng.StartCleanup(50 * time.Millisecond); err == nil {
		t.Fatal("expected ErrCleanupRunning")
	}
	if err := eng.StopCleanup(); err != nil {
		t.Fatalf("StopCleanup: %v", err)
	}
	if eng.CleanupRunning() {
		t.Fatal("expected stopped")
	}
}

func TestStartStopPersistence(t *testing.T) {
	eng := testEngine(t)
	if err := eng.StartPersistence(time.Hour, time.Hour); err != nil {
		t.Fatalf("StartPersistence: %v", err)
	}
	if !eng.Track.PeriodicSaveRunning() || !eng.Bans.PeriodicSaveRunning() || !eng.CleanupRunning() {
		t.Fatal("expected saves and cleanup running")
	}
	eng.RecordFailure("4.4.4.4", "test", time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC))
	if err := eng.StopPersistence(); err != nil {
		t.Fatalf("StopPersistence: %v", err)
	}
	if eng.Track.IsDirty() || eng.Bans.IsDirty() {
		t.Fatal("expected flush on stop")
	}
}
