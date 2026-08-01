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

func TestParseConfigInvalidPortExits(t *testing.T) {
	if os.Getenv("TEST_INVALID_PORT_EXIT") == "1" {
		os.Args = []string{"intern-auth-gateway", "serve"}
		_ = ParseConfig("Demo", "demo", "1.0.0", "abc")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestParseConfigInvalidPortExits", "-test.v")
	cmd.Env = append(os.Environ(),
		"TEST_INVALID_PORT_EXIT=1",
		"PORT=nope",
		"VERBOSE=",
		"HOST=",
		"ALLOWED_ORIGINS=",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, output: %s", out)
	}
	if !strings.Contains(string(out), "PORT") {
		t.Fatalf("expected PORT in output, got: %s", out)
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
