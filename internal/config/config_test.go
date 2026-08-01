package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"VERBOSE", "HOST", "PORT", "ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if ok && strings.HasPrefix(key, serviceEnvPrefix) {
			t.Setenv(key, "")
		}
	}
}

func TestParseConfigDefaults(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"intern-auth-gateway", "serve"}
	cfg := ParseConfig("Demo", "demo", "1.0.0", "abc")

	if cfg.Verbose {
		t.Fatal("expected Verbose false by default")
	}
	if cfg.Host != "0.0.0.0" {
		t.Fatalf("Host = %q, want 0.0.0.0", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.AllowedOrigins != "" {
		t.Fatalf("AllowedOrigins = %q, want empty", cfg.AllowedOrigins)
	}
	if cfg.Subcommand != "serve" {
		t.Fatalf("Subcommand = %q, want serve", cfg.Subcommand)
	}
	if len(cfg.Services) != 0 {
		t.Fatalf("Services len = %d, want 0", len(cfg.Services))
	}
}

func TestParseConfigBareRootDefaultsToServe(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"intern-auth-gateway"}
	cfg := ParseConfig("Demo", "demo", "1.0.0", "abc")
	if cfg.Subcommand != "serve" {
		t.Fatalf("Subcommand = %q, want serve", cfg.Subcommand)
	}
}

func TestParseConfigFlagsAndEnv(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9090")
	t.Setenv("ALLOWED_ORIGINS", "a.example.com, b.example.com")
	t.Setenv("VERBOSE", "true")

	os.Args = []string{"intern-auth-gateway", "serve", "--host", "10.0.0.1", "--port", "8081"}
	cfg := ParseConfig("Demo", "demo", "1.0.0", "abc")

	if !cfg.Verbose {
		t.Fatal("expected Verbose true from env")
	}
	if cfg.Host != "10.0.0.1" {
		t.Fatalf("Host = %q, want flag to win over env", cfg.Host)
	}
	if cfg.Port != 8081 {
		t.Fatalf("Port = %d, want flag to win over env", cfg.Port)
	}
	origins := cfg.AllowedOriginList()
	if len(origins) != 2 || origins[0] != "a.example.com" || origins[1] != "b.example.com" {
		t.Fatalf("AllowedOriginList = %#v", origins)
	}
}

func TestParseConfigServices(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	t.Setenv("SERVICE_test", "test.example.com/tester/$2a$14$AnhQELX1cqeO3YaLPOTWtOuPsKZgweRHrYLcqzQUcvokbVZmzNWrO")
	t.Setenv("SERVICE_intern", "*.intern.example.com/intern-user/$2a$14$54tdWftb4iOouKyfDyURPuI6rOIwcbjqKYfzOqYE0PyOcmVFnU1mM")
	// bcrypt-like hash that itself contains '/'
	t.Setenv("SERVICE_slash", "app.example.com/user/$2a$14$abc/def/ghi")

	os.Args = []string{"intern-auth-gateway", "serve"}
	cfg := ParseConfig("Demo", "demo", "1.0.0", "abc")

	if len(cfg.Services) != 3 {
		t.Fatalf("Services len = %d, want 3", len(cfg.Services))
	}

	testCred := cfg.Services["test"]
	if testCred.HostGlob != "test.example.com" || testCred.Username != "tester" {
		t.Fatalf("SERVICE_test = %#v", testCred)
	}
	if !strings.HasPrefix(testCred.PasswordHash, "$2a$14$") {
		t.Fatalf("unexpected password hash %q", testCred.PasswordHash)
	}

	slashCred := cfg.Services["slash"]
	if slashCred.PasswordHash != "$2a$14$abc/def/ghi" {
		t.Fatalf("slash hash = %q, want preserved inner slashes", slashCred.PasswordHash)
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

func TestParseConfigInvalidServiceExits(t *testing.T) {
	if os.Getenv("TEST_INVALID_SERVICE_EXIT") == "1" {
		os.Args = []string{"intern-auth-gateway", "serve"}
		_ = ParseConfig("Demo", "demo", "1.0.0", "abc")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestParseConfigInvalidServiceExits", "-test.v")
	cmd.Env = append(os.Environ(),
		"TEST_INVALID_SERVICE_EXIT=1",
		"SERVICE_bad=nope",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, output: %s", out)
	}
	if !strings.Contains(string(out), "SERVICE_bad") {
		t.Fatalf("expected SERVICE_bad in output, got: %s", out)
	}
}

func TestParseConfigVersionFlag(t *testing.T) {
	if os.Getenv("TEST_VERSION_EXIT") == "1" {
		os.Args = []string{"intern-auth-gateway", "--version"}
		_ = ParseConfig("Demo", "demo", "1.2.3", "deadbeef")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestParseConfigVersionFlag", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_VERSION_EXIT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Demo version 1.2.3, build deadbeef") {
		t.Fatalf("unexpected version output: %s", out)
	}
}
