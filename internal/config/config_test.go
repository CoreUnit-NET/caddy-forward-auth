package config

import (
	"errors"
	"os"
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
	cfg, err := ParseConfig("Demo", "demo")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

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
}

func TestParseConfigBareRootStartsServer(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"intern-auth-gateway"}
	cfg, err := ParseConfig("Demo", "demo")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.ShowVersion {
		t.Fatal("bare root must not set ShowVersion")
	}
	if cfg.Host != "0.0.0.0" {
		t.Fatalf("Host = %q, want defaults on bare root", cfg.Host)
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
	cfg, err := ParseConfig("Demo", "demo")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

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

func TestParseConfigInvalidPort(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	t.Setenv("PORT", "nope")
	os.Args = []string{"intern-auth-gateway", "serve"}
	_, err := ParseConfig("Demo", "demo")
	if err == nil {
		t.Fatal("expected error for invalid PORT")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("expected PORT in error, got: %v", err)
	}
}

func TestParseConfigVersionFlag(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"intern-auth-gateway", "--version"}
	cfg, err := ParseConfig("Demo", "demo")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if !cfg.ShowVersion {
		t.Fatal("expected ShowVersion true")
	}
}

func TestParseConfigHelpRequested(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"intern-auth-gateway", "--help"}
	_, err := ParseConfig("Demo", "demo")
	if !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("err = %v, want ErrHelpRequested", err)
	}
}
