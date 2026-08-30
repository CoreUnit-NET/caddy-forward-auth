package config

/*
Package config owns raw CLI flag and environment parsing (cobra).

It maps flags/env into a plain AppConfig struct only — no validation
beyond parse. Convert AppConfig into validated Settings via
internal/settings. Flags override env; missing .env is ignored at main.
*/

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const helpURL = "https://github.com/CoreUnit-NET/caddy-forward-auth"

type AppConfig struct {
	Verbose     bool
	ShowVersion bool

	Host           string
	Port           int
	AllowedOrigins string

	WhitelistEnabled      bool
	WhitelistPeriodHours  int
	WhitelistPath         string
	WhitelistOverridesBan bool

	FloodEnabled            bool
	FloodRetentionHours     int
	FloodCleanupMins        int
	FloodPath               string
	BanPath                 string
	DataSaveSecs            int
	FloodClearOnWhitelist   bool
	FloodCountNoCredentials bool
	FloodCountTempBanProbes bool

	FloodTier1Count      int
	FloodTier1WindowMins int
	FloodTier1BanMins    int
	FloodTier1Permanent  bool
	FloodTier2Count      int
	FloodTier2WindowMins int
	FloodTier2BanMins    int
	FloodTier2Permanent  bool
	FloodTier3Count      int
	FloodTier3WindowMins int
	FloodTier3BanMins    int
	FloodTier3Permanent  bool
	FloodTier4Count      int
	FloodTier4WindowMins int
	FloodTier4BanMins    int
	FloodTier4Permanent  bool
	FloodTier5Count      int
	FloodTier5WindowMins int
	FloodTier5BanMins    int
	FloodTier5Permanent  bool

	LogAuthSuccess bool
	LogWhitelisted bool
}

func defaultAppConfig() *AppConfig {
	return &AppConfig{
		Verbose:     false,
		ShowVersion: false,

		Host:           "0.0.0.0",
		Port:           8080,
		AllowedOrigins: "",

		WhitelistEnabled:      true,
		WhitelistPeriodHours:  48,
		WhitelistPath:         "./data/ipwhitelist.json",
		WhitelistOverridesBan: true,

		FloodEnabled:            true,
		FloodRetentionHours:     168,
		FloodCleanupMins:        60,
		FloodPath:               "./data/flood.json",
		BanPath:                 "./data/ban.json",
		DataSaveSecs:            30,
		FloodClearOnWhitelist:   false,
		FloodCountNoCredentials: true,
		FloodCountTempBanProbes: true,

		FloodTier1Count:      10,
		FloodTier1WindowMins: 2,
		FloodTier1BanMins:    3,
		FloodTier1Permanent:  false,
		FloodTier2Count:      60,
		FloodTier2WindowMins: 30,
		FloodTier2BanMins:    120,
		FloodTier2Permanent:  false,
		FloodTier3Count:      90,
		FloodTier3WindowMins: 60,
		FloodTier3BanMins:    0,
		FloodTier3Permanent:  true,
		FloodTier4Count:      120,
		FloodTier4WindowMins: 360,
		FloodTier4BanMins:    0,
		FloodTier4Permanent:  true,
		FloodTier5Count:      240,
		FloodTier5WindowMins: 10080,
		FloodTier5BanMins:    0,
		FloodTier5Permanent:  true,

		LogAuthSuccess: false,
		LogWhitelisted: false,
	}
}

func versionCommand(appConfig *AppConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			appConfig.ShowVersion = true
		},
	}
}

func serveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
}

func loadEnvVars(appConfig *AppConfig) error {
	loaders := []func() error{
		func() error {
			return envIsBool("VERBOSE", func(value bool) { appConfig.Verbose = value })
		},
		func() error { return envIsString("HOST", func(value string) { appConfig.Host = value }) },
		func() error { return envIsInt("PORT", func(value int) { appConfig.Port = value }) },
		func() error {
			return envIsString("ALLOWED_ORIGINS", func(value string) { appConfig.AllowedOrigins = value })
		},
		func() error {
			return envIsBool("WHITELIST_ENABLED", func(value bool) { appConfig.WhitelistEnabled = value })
		},
		func() error {
			return envIsInt("WHITELIST_PERIOD_HOURS", func(value int) { appConfig.WhitelistPeriodHours = value })
		},
		func() error {
			return envIsString("WHITELIST_PATH", func(value string) { appConfig.WhitelistPath = value })
		},
		func() error {
			return envIsBool("WHITELIST_OVERRIDES_BAN", func(value bool) { appConfig.WhitelistOverridesBan = value })
		},
		func() error { return envIsBool("FLOOD_ENABLED", func(value bool) { appConfig.FloodEnabled = value }) },
		func() error {
			return envIsInt("FLOOD_RETENTION_HOURS", func(value int) { appConfig.FloodRetentionHours = value })
		},
		func() error {
			return envIsInt("FLOOD_CLEANUP_MINS", func(value int) { appConfig.FloodCleanupMins = value })
		},
		func() error { return envIsString("FLOOD_PATH", func(value string) { appConfig.FloodPath = value }) },
		func() error { return envIsString("BAN_PATH", func(value string) { appConfig.BanPath = value }) },
		func() error { return envIsInt("DATA_SAVE_SECS", func(value int) { appConfig.DataSaveSecs = value }) },
		func() error {
			return envIsBool("FLOOD_CLEAR_ON_WHITELIST", func(value bool) { appConfig.FloodClearOnWhitelist = value })
		},
		func() error {
			return envIsBool("FLOOD_COUNT_NO_CREDENTIALS", func(value bool) { appConfig.FloodCountNoCredentials = value })
		},
		func() error {
			return envIsBool("FLOOD_COUNT_TEMP_BAN_PROBES", func(value bool) { appConfig.FloodCountTempBanProbes = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER1_COUNT", func(value int) { appConfig.FloodTier1Count = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER1_WINDOW_MINS", func(value int) { appConfig.FloodTier1WindowMins = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER1_BAN_MINS", func(value int) { appConfig.FloodTier1BanMins = value })
		},
		func() error {
			return envIsBool("FLOOD_TIER1_PERMANENT", func(value bool) { appConfig.FloodTier1Permanent = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER2_COUNT", func(value int) { appConfig.FloodTier2Count = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER2_WINDOW_MINS", func(value int) { appConfig.FloodTier2WindowMins = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER2_BAN_MINS", func(value int) { appConfig.FloodTier2BanMins = value })
		},
		func() error {
			return envIsBool("FLOOD_TIER2_PERMANENT", func(value bool) { appConfig.FloodTier2Permanent = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER3_COUNT", func(value int) { appConfig.FloodTier3Count = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER3_WINDOW_MINS", func(value int) { appConfig.FloodTier3WindowMins = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER3_BAN_MINS", func(value int) { appConfig.FloodTier3BanMins = value })
		},
		func() error {
			return envIsBool("FLOOD_TIER3_PERMANENT", func(value bool) { appConfig.FloodTier3Permanent = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER4_COUNT", func(value int) { appConfig.FloodTier4Count = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER4_WINDOW_MINS", func(value int) { appConfig.FloodTier4WindowMins = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER4_BAN_MINS", func(value int) { appConfig.FloodTier4BanMins = value })
		},
		func() error {
			return envIsBool("FLOOD_TIER4_PERMANENT", func(value bool) { appConfig.FloodTier4Permanent = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER5_COUNT", func(value int) { appConfig.FloodTier5Count = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER5_WINDOW_MINS", func(value int) { appConfig.FloodTier5WindowMins = value })
		},
		func() error {
			return envIsInt("FLOOD_TIER5_BAN_MINS", func(value int) { appConfig.FloodTier5BanMins = value })
		},
		func() error {
			return envIsBool("FLOOD_TIER5_PERMANENT", func(value bool) { appConfig.FloodTier5Permanent = value })
		},
		func() error {
			return envIsBool("LOG_AUTH_SUCCESS", func(value bool) { appConfig.LogAuthSuccess = value })
		},
		func() error {
			return envIsBool("LOG_WHITELISTED", func(value bool) { appConfig.LogWhitelisted = value })
		},
	}
	for _, load := range loaders {
		if err := load(); err != nil {
			return err
		}
	}
	return nil
}

func applyServeFlags(appConfig *AppConfig, cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&appConfig.Host, "host", appConfig.Host, "bind address (HOST)")
	cmd.PersistentFlags().IntVar(&appConfig.Port, "port", appConfig.Port, "listen port (PORT)")
	cmd.PersistentFlags().StringVar(&appConfig.AllowedOrigins, "allowed-origins", appConfig.AllowedOrigins, "CSV of allowed Origin hostnames/globs (ALLOWED_ORIGINS; same hostGlob rules as SERVICE_*)")

	cmd.PersistentFlags().BoolVar(&appConfig.WhitelistEnabled, "whitelist-enabled", appConfig.WhitelistEnabled, "enable temporary IP whitelist (WHITELIST_ENABLED)")
	cmd.PersistentFlags().IntVar(&appConfig.WhitelistPeriodHours, "whitelist-period-hours", appConfig.WhitelistPeriodHours, "whitelist active hours after each renewal (WHITELIST_PERIOD_HOURS)")
	cmd.PersistentFlags().StringVar(&appConfig.WhitelistPath, "whitelist-path", appConfig.WhitelistPath, "whitelist JSON path (WHITELIST_PATH)")
	cmd.PersistentFlags().BoolVar(&appConfig.WhitelistOverridesBan, "whitelist-overrides-ban", appConfig.WhitelistOverridesBan, "active whitelist bypasses bans (WHITELIST_OVERRIDES_BAN)")

	cmd.PersistentFlags().BoolVar(&appConfig.FloodEnabled, "flood-enabled", appConfig.FloodEnabled, "enable flood tracking and IP bans (FLOOD_ENABLED)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodRetentionHours, "flood-retention-hours", appConfig.FloodRetentionHours, "hours to keep flood events (FLOOD_RETENTION_HOURS)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodCleanupMins, "flood-cleanup-mins", appConfig.FloodCleanupMins, "minutes between flood/ban cleanup runs (FLOOD_CLEANUP_MINS)")
	cmd.PersistentFlags().StringVar(&appConfig.FloodPath, "flood-path", appConfig.FloodPath, "flood tracking JSON path (FLOOD_PATH)")
	cmd.PersistentFlags().StringVar(&appConfig.BanPath, "ban-path", appConfig.BanPath, "ban JSON path (BAN_PATH)")
	cmd.PersistentFlags().IntVar(&appConfig.DataSaveSecs, "data-save-secs", appConfig.DataSaveSecs, "seconds between dirty JSON saves (DATA_SAVE_SECS)")

	cmd.PersistentFlags().IntVar(&appConfig.FloodTier1Count, "flood-tier1-count", appConfig.FloodTier1Count, "tier-1 failure count before ban (FLOOD_TIER1_COUNT)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier1WindowMins, "flood-tier1-window-mins", appConfig.FloodTier1WindowMins, "tier-1 failure window minutes (FLOOD_TIER1_WINDOW_MINS)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier1BanMins, "flood-tier1-ban-mins", appConfig.FloodTier1BanMins, "tier-1 temp ban minutes (FLOOD_TIER1_BAN_MINS)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodTier1Permanent, "flood-tier1-permanent", appConfig.FloodTier1Permanent, "tier-1 permanent ban (FLOOD_TIER1_PERMANENT)")

	cmd.PersistentFlags().IntVar(&appConfig.FloodTier2Count, "flood-tier2-count", appConfig.FloodTier2Count, "tier-2 failure count before ban (FLOOD_TIER2_COUNT)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier2WindowMins, "flood-tier2-window-mins", appConfig.FloodTier2WindowMins, "tier-2 failure window minutes (FLOOD_TIER2_WINDOW_MINS)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier2BanMins, "flood-tier2-ban-mins", appConfig.FloodTier2BanMins, "tier-2 temp ban minutes (FLOOD_TIER2_BAN_MINS)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodTier2Permanent, "flood-tier2-permanent", appConfig.FloodTier2Permanent, "tier-2 permanent ban (FLOOD_TIER2_PERMANENT)")

	cmd.PersistentFlags().IntVar(&appConfig.FloodTier3Count, "flood-tier3-count", appConfig.FloodTier3Count, "tier-3 failure count before ban (FLOOD_TIER3_COUNT)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier3WindowMins, "flood-tier3-window-mins", appConfig.FloodTier3WindowMins, "tier-3 failure window minutes (FLOOD_TIER3_WINDOW_MINS)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier3BanMins, "flood-tier3-ban-mins", appConfig.FloodTier3BanMins, "tier-3 temp ban minutes (FLOOD_TIER3_BAN_MINS)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodTier3Permanent, "flood-tier3-permanent", appConfig.FloodTier3Permanent, "tier-3 permanent ban (FLOOD_TIER3_PERMANENT)")

	cmd.PersistentFlags().IntVar(&appConfig.FloodTier4Count, "flood-tier4-count", appConfig.FloodTier4Count, "tier-4 failure count before ban (FLOOD_TIER4_COUNT)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier4WindowMins, "flood-tier4-window-mins", appConfig.FloodTier4WindowMins, "tier-4 failure window minutes (FLOOD_TIER4_WINDOW_MINS)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier4BanMins, "flood-tier4-ban-mins", appConfig.FloodTier4BanMins, "tier-4 temp ban minutes (FLOOD_TIER4_BAN_MINS)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodTier4Permanent, "flood-tier4-permanent", appConfig.FloodTier4Permanent, "tier-4 permanent ban (FLOOD_TIER4_PERMANENT)")

	cmd.PersistentFlags().IntVar(&appConfig.FloodTier5Count, "flood-tier5-count", appConfig.FloodTier5Count, "tier-5 failure count before ban (FLOOD_TIER5_COUNT)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier5WindowMins, "flood-tier5-window-mins", appConfig.FloodTier5WindowMins, "tier-5 failure window minutes (FLOOD_TIER5_WINDOW_MINS)")
	cmd.PersistentFlags().IntVar(&appConfig.FloodTier5BanMins, "flood-tier5-ban-mins", appConfig.FloodTier5BanMins, "tier-5 temp ban minutes (FLOOD_TIER5_BAN_MINS)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodTier5Permanent, "flood-tier5-permanent", appConfig.FloodTier5Permanent, "tier-5 permanent ban (FLOOD_TIER5_PERMANENT)")

	cmd.PersistentFlags().BoolVar(&appConfig.FloodClearOnWhitelist, "flood-clear-on-whitelist", appConfig.FloodClearOnWhitelist, "clear flood failures when whitelisted probe hits (FLOOD_CLEAR_ON_WHITELIST)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodCountNoCredentials, "flood-count-no-credentials", appConfig.FloodCountNoCredentials, "count missing-credentials probes toward flood (FLOOD_COUNT_NO_CREDENTIALS)")
	cmd.PersistentFlags().BoolVar(&appConfig.FloodCountTempBanProbes, "flood-count-temp-ban-probes", appConfig.FloodCountTempBanProbes, "count probes while temp-banned toward flood (FLOOD_COUNT_TEMP_BAN_PROBES)")

	cmd.PersistentFlags().BoolVar(&appConfig.LogAuthSuccess, "log-auth-success", appConfig.LogAuthSuccess, "log successful Basic auth probes (LOG_AUTH_SUCCESS)")
	cmd.PersistentFlags().BoolVar(&appConfig.LogWhitelisted, "log-whitelisted", appConfig.LogWhitelisted, "log whitelisted probes (LOG_WHITELISTED)")
}

// ParseConfig loads env defaults, parses CLI flags/subcommands, and returns the app config.
// It returns ErrHelpRequested when the user asked for help (cobra has already printed it).
// Callers should handle ShowVersion and process exit themselves.
func ParseConfig(displayName, shortName string) (*AppConfig, error) {
	appConfig := defaultAppConfig()

	short := displayName + " answers Caddy forward_auth probes with HTTP Basic auth.\n" +
		"For more help, visit " + helpURL
	rootCmd := &cobra.Command{
		Use:   shortName,
		Short: short,
		Long: short + "\n" +
			"Running without a subcommand starts the HTTP server (same as '" + shortName + " serve').",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	rootCmd.PersistentFlags().BoolVarP(&appConfig.Verbose, "verbose", "b", appConfig.Verbose, "enable verbose mode (VERBOSE)")
	rootCmd.Flags().BoolVarP(&appConfig.ShowVersion, "version", "v", appConfig.ShowVersion, "print version")

	applyServeFlags(appConfig, rootCmd)

	if err := loadEnvVars(appConfig); err != nil {
		return nil, err
	}

	rootCmd.AddCommand(
		versionCommand(appConfig),
		serveCommand(),
	)

	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "",
		Hidden: true,
	})

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		return nil, err
	}

	if commandHelpRequested(cmd) {
		return nil, ErrHelpRequested
	}

	if appConfig.Verbose {
		fmt.Fprintln(os.Stderr, "Verbose mode enabled")
	}

	return appConfig, nil
}

func commandHelpRequested(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if helpFlag := c.Flags().Lookup("help"); helpFlag != nil && helpFlag.Changed {
			return true
		}
	}
	return false
}

// AllowedOriginList returns trimmed, non-empty origin entries from AllowedOrigins CSV.
// Entries may be exact hostnames or host globs (same rules as SERVICE_* hostGlob).
func (c *AppConfig) AllowedOriginList() []string {
	if strings.TrimSpace(c.AllowedOrigins) == "" {
		return nil
	}
	raw := strings.Split(c.AllowedOrigins, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
