package settings

/*
Package settings turns raw config into a validated, app-ready Settings struct.

Convert values into usable types (durations, flood tiers, origin lists).
Every field is validated before Settings is returned.
*/

import (
	"fmt"
	"strings"
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/config"
)

const (
	defaultWhitelistPeriodHours = 48
	defaultFloodRetentionHours  = 168
	defaultFloodCleanupMins     = 60
	defaultDataSaveSecs         = 30
)

// Settings is the validated runtime configuration for the auth server.
type Settings struct {
	Host           string
	Port           int
	AllowedOrigins []string
	Verbose        bool
	ShowVersion    bool

	Whitelist WhitelistSettings
	Flood     FloodSettings
	Log       LogSettings
}

// WhitelistSettings controls temporary IP whitelist behavior.
type WhitelistSettings struct {
	Enabled      bool
	PeriodHours  int
	Path         string
	OverridesBan bool
}

// FloodSettings controls flood tracking, temp/permanent bans, and persistence.
type FloodSettings struct {
	Enabled            bool
	RetentionHours     int
	CleanupMins        int
	FloodPath          string
	BanPath            string
	SaveSecs           int
	Tiers              []FloodTier
	ClearOnWhitelist   bool
	CountNoCredentials bool
	CountTempBanProbes bool
}

// FloodTier is one punishment threshold (count failures within window).
type FloodTier struct {
	ID         string
	Count      int
	WindowMins int
	BanMins    int
	Permanent  bool
}

// LogSettings controls auth probe logging.
type LogSettings struct {
	AuthSuccess bool
	Whitelisted bool
}

// FromAppConfig validates cfg and returns Settings.
func FromAppConfig(cfg *config.AppConfig) (*Settings, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	s := &Settings{
		Host:           strings.TrimSpace(cfg.Host),
		Port:           cfg.Port,
		AllowedOrigins: splitCSV(cfg.AllowedOrigins),
		Verbose:        cfg.Verbose,
		ShowVersion:    cfg.ShowVersion,
		Whitelist: WhitelistSettings{
			Enabled:      cfg.WhitelistEnabled,
			PeriodHours:  cfg.WhitelistPeriodHours,
			Path:         strings.TrimSpace(cfg.WhitelistPath),
			OverridesBan: cfg.WhitelistOverridesBan,
		},
		Flood: FloodSettings{
			Enabled:            cfg.FloodEnabled,
			RetentionHours:     cfg.FloodRetentionHours,
			CleanupMins:        cfg.FloodCleanupMins,
			FloodPath:          strings.TrimSpace(cfg.FloodPath),
			BanPath:            strings.TrimSpace(cfg.BanPath),
			SaveSecs:           cfg.DataSaveSecs,
			Tiers:              floodTiersFromConfig(cfg),
			ClearOnWhitelist:   cfg.FloodClearOnWhitelist,
			CountNoCredentials: cfg.FloodCountNoCredentials,
			CountTempBanProbes: cfg.FloodCountTempBanProbes,
		},
		Log: LogSettings{
			AuthSuccess: cfg.LogAuthSuccess,
			Whitelisted: cfg.LogWhitelisted,
		},
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Settings) validate() error {
	if s.Host == "" {
		return fmt.Errorf("host must not be empty")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", s.Port)
	}
	if s.Whitelist.PeriodHours < 1 {
		return fmt.Errorf("whitelist period hours must be >= 1, got %d", s.Whitelist.PeriodHours)
	}
	if s.Whitelist.Path == "" {
		return fmt.Errorf("whitelist path must not be empty")
	}
	if s.Flood.RetentionHours < 1 {
		return fmt.Errorf("flood retention hours must be >= 1, got %d", s.Flood.RetentionHours)
	}
	if s.Flood.CleanupMins < 1 {
		return fmt.Errorf("flood cleanup minutes must be >= 1, got %d", s.Flood.CleanupMins)
	}
	if s.Flood.FloodPath == "" {
		return fmt.Errorf("flood path must not be empty")
	}
	if s.Flood.BanPath == "" {
		return fmt.Errorf("ban path must not be empty")
	}
	if s.Flood.SaveSecs < 1 {
		return fmt.Errorf("data save seconds must be >= 1, got %d", s.Flood.SaveSecs)
	}
	if s.Flood.Enabled {
		if err := validateFloodTiers(s.Flood.Tiers); err != nil {
			return err
		}
	}
	return nil
}

// WhitelistPeriod returns how long whitelist entries stay active after renewal.
func (s *Settings) WhitelistPeriod() time.Duration {
	hours := defaultWhitelistPeriodHours
	if s != nil && s.Whitelist.PeriodHours > 0 {
		hours = s.Whitelist.PeriodHours
	}
	return time.Duration(hours) * time.Hour
}

// FloodRetention returns how long flood events are kept on disk.
func (s *Settings) FloodRetention() time.Duration {
	hours := defaultFloodRetentionHours
	if s != nil && s.Flood.RetentionHours > 0 {
		hours = s.Flood.RetentionHours
	}
	return time.Duration(hours) * time.Hour
}

// FloodCleanupInterval returns how often expired flood/ban entries are pruned.
func (s *Settings) FloodCleanupInterval() time.Duration {
	mins := defaultFloodCleanupMins
	if s != nil && s.Flood.CleanupMins > 0 {
		mins = s.Flood.CleanupMins
	}
	return time.Duration(mins) * time.Minute
}

// DataSaveInterval returns the periodic dirty-save tick for JSON bundles.
func (s *Settings) DataSaveInterval() time.Duration {
	secs := defaultDataSaveSecs
	if s != nil && s.Flood.SaveSecs > 0 {
		secs = s.Flood.SaveSecs
	}
	return time.Duration(secs) * time.Second
}

// LogSuccessfulAuth reports whether a successful Basic login should log.
func (s *Settings) LogSuccessfulAuth() bool {
	if s == nil {
		return false
	}
	return s.Verbose || s.Log.AuthSuccess
}

// LogWhitelistedProbe reports whether whitelisted probes should log.
func (s *Settings) LogWhitelistedProbe() bool {
	if s == nil {
		return false
	}
	return s.Log.Whitelisted
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func floodTiersFromConfig(cfg *config.AppConfig) []FloodTier {
	return []FloodTier{
		tierFromConfig(cfg.FloodTier1Count, cfg.FloodTier1WindowMins, cfg.FloodTier1BanMins, cfg.FloodTier1Permanent),
		tierFromConfig(cfg.FloodTier2Count, cfg.FloodTier2WindowMins, cfg.FloodTier2BanMins, cfg.FloodTier2Permanent),
		tierFromConfig(cfg.FloodTier3Count, cfg.FloodTier3WindowMins, cfg.FloodTier3BanMins, cfg.FloodTier3Permanent),
		tierFromConfig(cfg.FloodTier4Count, cfg.FloodTier4WindowMins, cfg.FloodTier4BanMins, cfg.FloodTier4Permanent),
		tierFromConfig(cfg.FloodTier5Count, cfg.FloodTier5WindowMins, cfg.FloodTier5BanMins, cfg.FloodTier5Permanent),
	}
}

func tierFromConfig(count, windowMins, banMins int, permanent bool) FloodTier {
	return FloodTier{
		ID:         FormatTierID(count, windowMins),
		Count:      count,
		WindowMins: windowMins,
		BanMins:    banMins,
		Permanent:  permanent,
	}
}

// FormatTierID labels a tier in ban.json (for example "10/2m", "120/6h").
func FormatTierID(count, windowMins int) string {
	if windowMins >= 60 && windowMins%60 == 0 {
		return fmt.Sprintf("%d/%dh", count, windowMins/60)
	}
	return fmt.Sprintf("%d/%dm", count, windowMins)
}

func validateFloodTiers(tiers []FloodTier) error {
	if len(tiers) == 0 {
		return fmt.Errorf("flood enabled but no tiers configured")
	}
	for _, tier := range tiers {
		if tier.Count < 1 {
			return fmt.Errorf("flood tier %s count must be >= 1, got %d", tier.ID, tier.Count)
		}
		if tier.WindowMins < 1 {
			return fmt.Errorf("flood tier %s window minutes must be >= 1, got %d", tier.ID, tier.WindowMins)
		}
		if !tier.Permanent && tier.BanMins < 1 {
			return fmt.Errorf("flood tier %s ban minutes must be >= 1 for temp bans, got %d", tier.ID, tier.BanMins)
		}
	}
	return nil
}
