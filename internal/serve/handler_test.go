package serve

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/auth"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/flood"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodban"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/ipwhitelist"
)

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.New(io.Discard, "", 0)
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return string(hash)
}

func testOrigins() []string {
	return []string{"intern-auth.example.com", "localhost"}
}

func testServices(t *testing.T) map[string]auth.ServiceCred {
	t.Helper()
	return map[string]auth.ServiceCred{
		"test": {
			Name:         "test",
			HostGlob:     "test.example.com",
			Username:     "tester",
			PasswordHash: mustHash(t, "secret"),
		},
		"intern": {
			Name:         "intern",
			HostGlob:     "*.intern.example.com",
			Username:     "intern-user",
			PasswordHash: mustHash(t, "intern-secret"),
		},
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewHandler(testLogger(t), true, testOrigins(), testServices(t), "test-realm", nil, nil)
}

func testHandlerWithLog(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	return NewHandler(logger, true, testOrigins(), testServices(t), "test-realm", nil, nil), &buf
}

func basicHeader(user, pass string) string {
	token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return "Basic " + token
}

func TestAuthProbeSuccess(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Host = "auth.local"
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))
	req.Header.Set("Origin", "https://intern-auth.example.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Remote-User"); got != "tester" {
		t.Fatalf("Remote-User = %q, want tester", got)
	}
	assertAuthLog(t, buf.String(),
		"status=200",
		"method=GET",
		"path=/auth",
		"host=test.example.com",
		"origin=https://intern-auth.example.com",
		"service=test",
		"user=tester",
		"reason=ok",
	)
}

func TestAuthProbeUnauthorized(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "wrong"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rr.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate header")
	}
	assertAuthLog(t, buf.String(), "status=401", "path=/auth", "host=test.example.com", "user=tester", "reason=auth_failed")
}

func TestAuthProbeOriginRejected(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))
	req.Header.Set("Origin", "https://evil.example.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	assertAuthLog(t, buf.String(),
		"status=403",
		"method=GET",
		"host=test.example.com",
		"origin=https://evil.example.com",
		"ip=203.0.113.50",
		"reason=origin_not_allowed",
	)
}

func TestAuthProbeOriginNullRejected(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Origin", "null")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	assertAuthLog(t, buf.String(), "origin=null", "reason=origin_not_allowed")
}

func TestAuthProbeOriginGlobAllowed(t *testing.T) {
	h := NewHandler(
		testLogger(t),
		true,
		[]string{"*.intern.example.com"},
		testServices(t),
		"test-realm",
		nil,
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))
	req.Header.Set("Origin", "https://ai-dashboard.intern.example.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthProbeWildcardHost(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Host", "api.intern.example.com")
	req.Header.Set("Authorization", basicHeader("intern-user", "intern-secret"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	assertAuthLog(t, buf.String(), "status=200", "path=/", "service=intern", "user=intern-user", "reason=ok")
}

func TestAuthProbeNoMatchingService(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "unknown.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	assertAuthLog(t, buf.String(), "status=401", "reason=no_service", "service=-")
}

func TestAuthProbeUnknownPathNotFound(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	assertAuthLog(t, buf.String(), "status=404", "path=/foo", "reason=not_found")
}

func TestAuthProbeNoCredentials(t *testing.T) {
	h, buf := testHandlerWithLog(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	assertAuthLog(t, buf.String(), "status=401", "reason=no_credentials", "user=-")
}

func TestLogVerboseConfigIncludesPasswordHash(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	services := testServices(t)
	logVerboseConfig(logger, services, []string{"localhost"})

	out := buf.String()
	if !strings.Contains(out, "passwordHash=") {
		t.Fatalf("expected passwordHash in verbose dump, got: %s", out)
	}
	if !strings.Contains(out, "service intern ->") || !strings.Contains(out, "service test ->") {
		t.Fatalf("expected sorted service lines, got: %s", out)
	}
	if !strings.Contains(out, "allowed origin localhost") {
		t.Fatalf("expected allowed origin line, got: %s", out)
	}
	// Ensure hashes from both services appear (not redacted).
	for _, cred := range services {
		if !strings.Contains(out, cred.PasswordHash) {
			t.Fatalf("expected hash %q in verbose dump", cred.PasswordHash)
		}
	}
}

func TestAuthProbeWhitelistedSkipsBasic(t *testing.T) {
	dir := t.TempDir()
	wl, err := ipwhitelist.Open(filepath.Join(dir, "ipwhitelist.json"))
	if err != nil {
		t.Fatalf("whitelist open: %v", err)
	}
	wl.UpsertNow("203.0.113.10")

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := NewHandler(logger, true, testOrigins(), testServices(t), "test-realm", wl, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
	assertAuthLog(t, buf.String(), "status=200", "ip=203.0.113.10", "reason=whitelisted")
}

func TestAuthProbeSuccessSilentWithoutVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := NewHandler(logger, false, testOrigins(), testServices(t), "test-realm", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no success log without verbose, got: %q", buf.String())
	}
}

func TestAuthProbeRejectionLoggedWithoutVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := NewHandler(logger, false, testOrigins(), testServices(t), "test-realm", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rr.Code)
	}
	assertAuthLog(t, buf.String(), "origin=https://evil.example.com", "reason=origin_not_allowed")
}

func TestAuthProbeRecordsFloodOnAuthFailed(t *testing.T) {
	dir := t.TempDir()
	eng, err := flood.Open(filepath.Join(dir, "flood.json"), filepath.Join(dir, "ban.json"))
	if err != nil {
		t.Fatalf("flood open: %v", err)
	}

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := NewHandler(logger, true, testOrigins(), testServices(t), "test-realm", nil, eng)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("X-Forwarded-For", "198.51.100.20")
	req.Header.Set("Authorization", basicHeader("tester", "wrong"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rr.Code)
	}
	if got := len(eng.Track.Entries()); got != 1 {
		t.Fatalf("flood entries=%d, want 1", got)
	}
	assertAuthLog(t, buf.String(), "reason=auth_failed", "ip=198.51.100.20")
}

func TestAuthProbePermanentBan(t *testing.T) {
	dir := t.TempDir()
	eng, err := flood.Open(filepath.Join(dir, "flood.json"), filepath.Join(dir, "ban.json"))
	if err != nil {
		t.Fatalf("flood open: %v", err)
	}
	eng.Bans.Upsert(floodban.Ban{
		IP:        "198.51.100.30",
		Permanent: true,
		BannedAt:  time.Now().UTC(),
		Rule:      "manual",
	})

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := NewHandler(logger, true, testOrigins(), testServices(t), "test-realm", nil, eng)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("X-Forwarded-For", "198.51.100.30")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rr.Code)
	}
	assertAuthLog(t, buf.String(), "reason=banned", "ip=198.51.100.30")
}

func TestAtom(t *testing.T) {
	if got := atom(""); got != "-" {
		t.Fatalf("atom empty = %q, want -", got)
	}
	if got := atom("ok"); got != "ok" {
		t.Fatalf("atom plain = %q, want ok", got)
	}
	if got := atom("a b"); got != `"a b"` {
		t.Fatalf("atom spaced = %q, want quoted", got)
	}
}

func assertAuthLog(t *testing.T, got string, wantParts ...string) {
	t.Helper()
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("auth log missing %q; got: %q", part, got)
		}
	}
}
