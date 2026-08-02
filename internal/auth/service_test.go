package auth

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
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

func mustHashWithSlash(t *testing.T) string {
	t.Helper()
	for i := 0; i < 200; i++ {
		hash, err := bcrypt.GenerateFromPassword([]byte(fmt.Sprintf("slash-pw-%d", i)), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		if strings.Contains(string(hash), "/") {
			return string(hash)
		}
	}
	t.Fatal("could not generate bcrypt hash containing '/'")
	return ""
}

func TestLoadServicesFromEnv(t *testing.T) {
	clearServiceEnv(t)

	slashHash := mustHashWithSlash(t)
	t.Setenv("SERVICE_test", "test.example.com/tester/"+mustHash(t, "secret"))
	t.Setenv("SERVICE_intern", "*.intern.example.com/intern-user/"+mustHash(t, "intern"))
	t.Setenv("SERVICE_slash", "app.example.com/user/"+slashHash)

	services, err := LoadServicesFromEnv()
	if err != nil {
		t.Fatalf("LoadServicesFromEnv: %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("Services len = %d, want 3", len(services))
	}

	testCred := services["test"]
	if testCred.Name != "test" || testCred.HostGlob != "test.example.com" || testCred.Username != "tester" {
		t.Fatalf("SERVICE_test = %#v", testCred)
	}
	if _, err := bcrypt.Cost([]byte(testCred.PasswordHash)); err != nil {
		t.Fatalf("unexpected password hash %q: %v", testCred.PasswordHash, err)
	}

	slashCred := services["slash"]
	if slashCred.PasswordHash != slashHash {
		t.Fatalf("slash hash = %q, want preserved inner slashes", slashCred.PasswordHash)
	}
}

func TestLoadServicesDuplicateUsername(t *testing.T) {
	clearServiceEnv(t)
	t.Setenv("SERVICE_a", "a.example.com/sameuser/"+mustHash(t, "a"))
	t.Setenv("SERVICE_b", "b.example.com/sameuser/"+mustHash(t, "b"))

	_, err := LoadServicesFromEnv()
	if err == nil {
		t.Fatal("expected duplicate username error")
	}
	if !strings.Contains(err.Error(), "sameuser") {
		t.Fatalf("expected username in error, got: %v", err)
	}
}

func TestLoadServicesRequiresAtLeastOne(t *testing.T) {
	clearServiceEnv(t)
	_, err := LoadServicesFromEnv()
	if err == nil {
		t.Fatal("expected error when no SERVICE_* entries exist")
	}
	if !strings.Contains(err.Error(), "no SERVICE_*") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadServicesRejectsInvalidBcrypt(t *testing.T) {
	clearServiceEnv(t)
	t.Setenv("SERVICE_bad", "bad.example.com/user/not-a-bcrypt-hash")

	_, err := LoadServicesFromEnv()
	if err == nil {
		t.Fatal("expected invalid passwordHash error")
	}
	if !strings.Contains(err.Error(), "passwordHash") {
		t.Fatalf("unexpected error: %v", err)
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

func TestParseServiceValueKeepsHashSlashes(t *testing.T) {
	cred, err := parseServiceValue("SERVICE_slash", "app.example.com/user/$2a$14$abc/def/ghi")
	if err != nil {
		t.Fatalf("parseServiceValue: %v", err)
	}
	if cred.PasswordHash != "$2a$14$abc/def/ghi" {
		t.Fatalf("hash = %q", cred.PasswordHash)
	}
}

func TestSortedServiceNames(t *testing.T) {
	names := SortedServiceNames(map[string]ServiceCred{
		"zeta":  {},
		"alpha": {},
		"mid":   {},
	})
	want := []string{"alpha", "mid", "zeta"}
	if len(names) != len(want) {
		t.Fatalf("len = %d, want %d", len(names), len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
}
