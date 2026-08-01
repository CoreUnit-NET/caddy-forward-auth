package auth

import (
	"os"
	"strings"
	"testing"
)

func clearServiceEnv(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if ok && strings.HasPrefix(key, serviceEnvPrefix) {
			t.Setenv(key, "")
		}
	}
}

func TestLoadServicesFromEnv(t *testing.T) {
	clearServiceEnv(t)

	t.Setenv("SERVICE_test", "test.example.com/tester/$2a$14$AnhQELX1cqeO3YaLPOTWtOuPsKZgweRHrYLcqzQUcvokbVZmzNWrO")
	t.Setenv("SERVICE_intern", "*.intern.example.com/intern-user/$2a$14$54tdWftb4iOouKyfDyURPuI6rOIwcbjqKYfzOqYE0PyOcmVFnU1mM")
	t.Setenv("SERVICE_slash", "app.example.com/user/$2a$14$abc/def/ghi")

	services, err := LoadServicesFromEnv()
	if err != nil {
		t.Fatalf("LoadServicesFromEnv: %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("Services len = %d, want 3", len(services))
	}

	testCred := services["test"]
	if testCred.HostGlob != "test.example.com" || testCred.Username != "tester" {
		t.Fatalf("SERVICE_test = %#v", testCred)
	}
	if !strings.HasPrefix(testCred.PasswordHash, "$2a$14$") {
		t.Fatalf("unexpected password hash %q", testCred.PasswordHash)
	}

	slashCred := services["slash"]
	if slashCred.PasswordHash != "$2a$14$abc/def/ghi" {
		t.Fatalf("slash hash = %q, want preserved inner slashes", slashCred.PasswordHash)
	}
}

func TestLoadServicesDuplicateUsername(t *testing.T) {
	clearServiceEnv(t)
	t.Setenv("SERVICE_a", "a.example.com/sameuser/hash1")
	t.Setenv("SERVICE_b", "b.example.com/sameuser/hash2")

	_, err := LoadServicesFromEnv()
	if err == nil {
		t.Fatal("expected duplicate username error")
	}
	if !strings.Contains(err.Error(), "sameuser") {
		t.Fatalf("expected username in error, got: %v", err)
	}
}

func TestParseServiceValueErrors(t *testing.T) {
	_, err := parseServiceValue("SERVICE_bad", "only-one-part")
	if err == nil {
		t.Fatal("expected error for missing separators")
	}
	_, err = parseServiceValue("SERVICE_bad", "a/b")
	if err == nil {
		t.Fatal("expected error for one separator")
	}
	_, err = parseServiceValue("SERVICE_bad", "/user/hash")
	if err == nil {
		t.Fatal("expected error for empty host glob")
	}
}
