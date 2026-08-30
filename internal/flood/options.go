package flood

import (
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/settings"
)

// Options configures flood tracking, bans, and background persistence.
type Options struct {
	FloodPath          string
	BanPath            string
	Rules              []Rule
	Retention          time.Duration
	SaveInterval       time.Duration
	CleanupInterval    time.Duration
	CountTempBanProbes bool
}

// Rule is one flood threshold that may introduce a ban.
type Rule struct {
	ID           string
	Count        int
	Window       time.Duration
	TempDuration time.Duration
	Permanent    bool
}

// DefaultOptions returns options matching the historical hardcoded defaults.
func DefaultOptions() Options {
	return Options{
		FloodPath:          "./data/flood.json",
		BanPath:            "./data/ban.json",
		Rules:              defaultRules(),
		Retention:          168 * time.Hour,
		SaveInterval:       30 * time.Second,
		CleanupInterval:    time.Hour,
		CountTempBanProbes: true,
	}
}

// OptionsFromSettings builds flood Options from validated settings.
func OptionsFromSettings(s *settings.Settings) Options {
	opts := DefaultOptions()
	if s == nil {
		return opts
	}
	opts.FloodPath = s.Flood.FloodPath
	opts.BanPath = s.Flood.BanPath
	opts.Retention = s.FloodRetention()
	opts.SaveInterval = s.DataSaveInterval()
	opts.CleanupInterval = s.FloodCleanupInterval()
	opts.CountTempBanProbes = s.Flood.CountTempBanProbes
	opts.Rules = rulesFromSettings(s.Flood.Tiers)
	return opts
}

func rulesFromSettings(tiers []settings.FloodTier) []Rule {
	if len(tiers) == 0 {
		return defaultRules()
	}
	out := make([]Rule, 0, len(tiers))
	for _, tier := range tiers {
		rule := Rule{
			ID:        tier.ID,
			Count:     tier.Count,
			Window:    time.Duration(tier.WindowMins) * time.Minute,
			Permanent: tier.Permanent,
		}
		if !tier.Permanent {
			rule.TempDuration = time.Duration(tier.BanMins) * time.Minute
		}
		out = append(out, rule)
	}
	return out
}
