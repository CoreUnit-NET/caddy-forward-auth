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
		"WHITELIST_ENABLED", "WHITELIST_PERIOD_HOURS", "WHITELIST_PATH", "WHITELIST_OVERRIDES_BAN",
		"FLOOD_ENABLED", "FLOOD_RETENTION_HOURS", "FLOOD_CLEANUP_MINS", "FLOOD_PATH", "BAN_PATH",
		"DATA_SAVE_SECS", "FLOOD_CLEAR_ON_WHITELIST", "FLOOD_COUNT_NO_CREDENTIALS", "FLOOD_COUNT_TEMP_BAN_PROBES",
		"FLOOD_TIER1_COUNT", "FLOOD_TIER1_WINDOW_MINS", "FLOOD_TIER1_BAN_MINS", "FLOOD_TIER1_PERMANENT",
		"LOG_AUTH_SUCCESS", "LOG_WHITELISTED",
	} {
		t.Setenv(key, "")
	}
}

func TestParseConfigDefaults(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"caddy-forward-auth", "serve"}
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
	if !cfg.WhitelistEnabled {
		t.Fatal("expected WhitelistEnabled true by default")
	}
	if cfg.WhitelistPeriodHours != 48 {
		t.Fatalf("WhitelistPeriodHours = %d, want 48", cfg.WhitelistPeriodHours)
	}
	if !cfg.FloodEnabled {
		t.Fatal("expected FloodEnabled true by default")
	}
	if !cfg.WhitelistOverridesBan {
		t.Fatal("expected WhitelistOverridesBan true by default")
	}
	if !cfg.FloodCountNoCredentials {
		t.Fatal("expected FloodCountNoCredentials true by default")
	}
}

func TestParseConfigBareRootStartsServer(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	os.Args = []string{"caddy-forward-auth"}
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

	os.Args = []string{"caddy-forward-auth", "serve", "--host", "10.0.0.1", "--port", "8081"}
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
	os.Args = []string{"caddy-forward-auth", "serve"}
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

	os.Args = []string{"caddy-forward-auth", "--version"}
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

	os.Args = []string{"caddy-forward-auth", "--help"}
	_, err := ParseConfig("Demo", "demo")
	if !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("err = %v, want ErrHelpRequested", err)
	}
}

func TestParseConfigPolicyEnv(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	clearConfigEnv(t)

	t.Setenv("WHITELIST_OVERRIDES_BAN", "false")
	t.Setenv("FLOOD_CLEAR_ON_WHITELIST", "true")
	t.Setenv("FLOOD_COUNT_NO_CREDENTIALS", "false")
	t.Setenv("FLOOD_TIER1_COUNT", "5")
	t.Setenv("LOG_WHITELISTED", "true")

	os.Args = []string{"caddy-forward-auth", "serve"}
	cfg, err := ParseConfig("Demo", "demo")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.WhitelistOverridesBan {
		t.Fatal("expected WHITELIST_OVERRIDES_BAN=false from env")
	}
	if !cfg.FloodClearOnWhitelist {
		t.Fatal("expected FLOOD_CLEAR_ON_WHITELIST=true from env")
	}
	if cfg.FloodCountNoCredentials {
		t.Fatal("expected FLOOD_COUNT_NO_CREDENTIALS=false from env")
	}
	if cfg.FloodTier1Count != 5 {
		t.Fatalf("FloodTier1Count = %d, want 5", cfg.FloodTier1Count)
	}
	if !cfg.LogWhitelisted {
		t.Fatal("expected LOG_WHITELISTED=true from env")
	}
}
