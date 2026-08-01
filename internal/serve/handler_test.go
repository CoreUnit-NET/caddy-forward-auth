package serve

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/auth"
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
			HostGlob:     "test.example.com",
			Username:     "tester",
			PasswordHash: mustHash(t, "secret"),
		},
		"intern": {
			HostGlob:     "*.intern.example.com",
			Username:     "intern-user",
			PasswordHash: mustHash(t, "intern-secret"),
		},
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewHandler(testLogger(t), false, testOrigins(), testServices(t), "test-realm")
}

func basicHeader(user, pass string) string {
	token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return "Basic " + token
}

func TestAuthProbeSuccess(t *testing.T) {
	h := testHandler(t)
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
}

func TestAuthProbeUnauthorized(t *testing.T) {
	h := testHandler(t)
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
}

func TestAuthProbeOriginRejected(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))
	req.Header.Set("Origin", "https://evil.example.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestAuthProbeWildcardHost(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Host", "api.intern.example.com")
	req.Header.Set("Authorization", basicHeader("intern-user", "intern-secret"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthProbeNoMatchingService(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "unknown.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAuthProbeUnknownPathNotFound(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	req.Header.Set("X-Forwarded-Host", "test.example.com")
	req.Header.Set("Authorization", basicHeader("tester", "secret"))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
