package config

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
}

func defaultAppConfig() *AppConfig {
	return &AppConfig{
		Verbose:     false,
		ShowVersion: false,

		Host:           "0.0.0.0",
		Port:           8080,
		AllowedOrigins: "",
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
	if err := envIsBool("VERBOSE", func(value bool) {
		appConfig.Verbose = value
	}); err != nil {
		return err
	}
	if err := envIsString("HOST", func(value string) {
		appConfig.Host = value
	}); err != nil {
		return err
	}
	if err := envIsInt("PORT", func(value int) {
		appConfig.Port = value
	}); err != nil {
		return err
	}
	if err := envIsString("ALLOWED_ORIGINS", func(value string) {
		appConfig.AllowedOrigins = value
	}); err != nil {
		return err
	}
	return nil
}

func applyServeFlags(appConfig *AppConfig, cmd *cobra.Command) {
	// Persistent so bare root and `serve` share one definition (explorer-mcp style).
	cmd.PersistentFlags().StringVar(&appConfig.Host, "host", appConfig.Host, "bind address (HOST)")
	cmd.PersistentFlags().IntVar(&appConfig.Port, "port", appConfig.Port, "listen port (PORT)")
	cmd.PersistentFlags().StringVar(&appConfig.AllowedOrigins, "allowed-origins", appConfig.AllowedOrigins, "CSV of allowed Origin hostnames/globs (ALLOWED_ORIGINS; same hostGlob rules as SERVICE_*)")
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

	// Disable cobra's help subcommand so `help` is not a valid command.
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
