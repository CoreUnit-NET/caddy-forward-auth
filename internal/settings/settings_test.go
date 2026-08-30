package settings

import (
	"testing"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/config"
)

func defaultConfig() *config.AppConfig {
	return &config.AppConfig{
		Host:                    "127.0.0.1",
		Port:                    8080,
		AllowedOrigins:          "a.example.com,b.example.com",
		WhitelistEnabled:        true,
		WhitelistPeriodHours:    48,
		WhitelistPath:           "./data/ipwhitelist.json",
		WhitelistOverridesBan:   true,
		FloodEnabled:            true,
		FloodRetentionHours:     168,
		FloodCleanupMins:        60,
		FloodPath:               "./data/flood.json",
		BanPath:                 "./data/ban.json",
		DataSaveSecs:            30,
		FloodClearOnWhitelist:   false,
		FloodCountNoCredentials: true,
		FloodCountTempBanProbes: true,
		FloodTier1Count:         10,
		FloodTier1WindowMins:    2,
		FloodTier1BanMins:       3,
		FloodTier1Permanent:     false,
		FloodTier2Count:         60,
		FloodTier2WindowMins:    30,
		FloodTier2BanMins:       120,
		FloodTier2Permanent:     false,
		FloodTier3Count:         90,
		FloodTier3WindowMins:    60,
		FloodTier3BanMins:       0,
		FloodTier3Permanent:     true,
		FloodTier4Count:         120,
		FloodTier4WindowMins:    360,
		FloodTier4BanMins:       0,
		FloodTier4Permanent:     true,
		FloodTier5Count:         240,
		FloodTier5WindowMins:    10080,
		FloodTier5BanMins:       0,
		FloodTier5Permanent:     true,
		LogAuthSuccess:          false,
		LogWhitelisted:          false,
	}
}

func TestFromAppConfigDefaults(t *testing.T) {
	st, err := FromAppConfig(defaultConfig())
	if err != nil {
		t.Fatalf("FromAppConfig: %v", err)
	}
	if st.Host != "127.0.0.1" {
		t.Fatalf("Host = %q", st.Host)
	}
	if len(st.AllowedOrigins) != 2 {
		t.Fatalf("AllowedOrigins = %#v", st.AllowedOrigins)
	}
	if !st.Whitelist.OverridesBan {
		t.Fatal("expected Whitelist.OverridesBan true by default")
	}
	if len(st.Flood.Tiers) != 5 {
		t.Fatalf("tiers = %d, want 5", len(st.Flood.Tiers))
	}
}

func TestFromAppConfigDurationHelpers(t *testing.T) {
	cfg := defaultConfig()
	cfg.WhitelistPeriodHours = 24
	cfg.FloodRetentionHours = 72
	cfg.FloodCleanupMins = 15
	cfg.DataSaveSecs = 10

	st, err := FromAppConfig(cfg)
	if err != nil {
		t.Fatalf("FromAppConfig: %v", err)
	}
	if got := st.WhitelistPeriod(); got != 24*time.Hour {
		t.Fatalf("WhitelistPeriod = %v, want 24h", got)
	}
	if got := st.FloodRetention(); got != 72*time.Hour {
		t.Fatalf("FloodRetention = %v, want 72h", got)
	}
	if got := st.FloodCleanupInterval(); got != 15*time.Minute {
		t.Fatalf("FloodCleanupInterval = %v, want 15m", got)
	}
	if got := st.DataSaveInterval(); got != 10*time.Second {
		t.Fatalf("DataSaveInterval = %v, want 10s", got)
	}
}

func TestFromAppConfigLogSuccessfulAuth(t *testing.T) {
	cfg := defaultConfig()
	st, err := FromAppConfig(cfg)
	if err != nil {
		t.Fatalf("FromAppConfig: %v", err)
	}
	if st.LogSuccessfulAuth() {
		t.Fatal("expected LogSuccessfulAuth false without verbose or LOG_AUTH_SUCCESS")
	}

	cfg.Verbose = true
	st, err = FromAppConfig(cfg)
	if err != nil {
		t.Fatalf("FromAppConfig verbose: %v", err)
	}
	if !st.LogSuccessfulAuth() {
		t.Fatal("expected LogSuccessfulAuth true when verbose")
	}

	cfg.Verbose = false
	cfg.LogAuthSuccess = true
	st, err = FromAppConfig(cfg)
	if err != nil {
		t.Fatalf("FromAppConfig log auth success: %v", err)
	}
	if !st.LogSuccessfulAuth() {
		t.Fatal("expected LogSuccessfulAuth true when LOG_AUTH_SUCCESS")
	}
}

func TestFormatTierID(t *testing.T) {
	tests := []struct {
		count, windowMins int
		want              string
	}{
		{10, 2, "10/2m"},
		{60, 30, "60/30m"},
		{120, 360, "120/6h"},
		{240, 10080, "240/168h"},
	}
	for _, tc := range tests {
		if got := FormatTierID(tc.count, tc.windowMins); got != tc.want {
			t.Fatalf("FormatTierID(%d, %d) = %q, want %q", tc.count, tc.windowMins, got, tc.want)
		}
	}
}

func TestFromAppConfigTierIDs(t *testing.T) {
	st, err := FromAppConfig(defaultConfig())
	if err != nil {
		t.Fatalf("FromAppConfig: %v", err)
	}
	want := []string{"10/2m", "60/30m", "90/1h", "120/6h", "240/168h"}
	for i, tier := range st.Flood.Tiers {
		if tier.ID != want[i] {
			t.Fatalf("tier %d ID = %q, want %q", i+1, tier.ID, want[i])
		}
	}
}

func TestFromAppConfigValidation(t *testing.T) {
	cfg := defaultConfig()
	cfg.Port = 0
	if _, err := FromAppConfig(cfg); err == nil {
		t.Fatal("expected error for invalid port")
	}

	cfg = defaultConfig()
	cfg.FloodTier1BanMins = 0
	cfg.FloodTier1Permanent = false
	if _, err := FromAppConfig(cfg); err == nil {
		t.Fatal("expected error for temp tier without ban minutes")
	}

	cfg = defaultConfig()
	cfg.FloodEnabled = false
	if _, err := FromAppConfig(cfg); err != nil {
		t.Fatalf("flood disabled should skip tier validation: %v", err)
	}
}
