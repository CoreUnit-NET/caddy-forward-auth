package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type AppConfig struct {
	Verbose     bool
	ShowVersion bool
	Subcommand  string

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
		Short: "Prints version message",
		Run: func(cmd *cobra.Command, args []string) {
			appConfig.ShowVersion = true
		},
	}
}

func serveCommand(appConfig *AppConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Run: func(cmd *cobra.Command, args []string) {
			appConfig.Subcommand = "serve"
		},
	}
}

func loadEnvVars(appConfig *AppConfig) error {
	if err := EnvIsBool("VERBOSE", func(value bool) {
		appConfig.Verbose = value
	}); err != nil {
		return err
	}
	if err := EnvIsString("HOST", func(value string) {
		appConfig.Host = value
	}); err != nil {
		return err
	}
	if err := EnvIsInt("PORT", func(value int) {
		appConfig.Port = value
	}); err != nil {
		return err
	}
	if err := EnvIsString("ALLOWED_ORIGINS", func(value string) {
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
	cmd.PersistentFlags().StringVar(&appConfig.AllowedOrigins, "allowed-origins", appConfig.AllowedOrigins, "CSV of allowed Origin hostnames (ALLOWED_ORIGINS)")
}

func ParseConfig(
	displayName string,
	shortName string,
	version string,
	commit string,
) *AppConfig {
	appConfig := defaultAppConfig()

	rootCmd := &cobra.Command{
		Use: shortName,
		Short: displayName + " is a basic-auth gateway for caddy forward_auth probes.\n" +
			"For more help, visit https://github.com/NobleMajo/intern-auth-gateway",
		Long: displayName + " is a basic-auth gateway for caddy forward_auth probes.\n" +
			"Running without a subcommand starts the HTTP server (same as '" + shortName + " serve').\n" +
			"For more help, visit https://github.com/NobleMajo/intern-auth-gateway",
		Run: func(cmd *cobra.Command, args []string) {
			appConfig.Subcommand = "serve"
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&appConfig.Verbose, "verbose", "b", appConfig.Verbose, "enable verbose mode (VERBOSE)")
	rootCmd.Flags().BoolVarP(&appConfig.ShowVersion, "version", "v", appConfig.ShowVersion, "prints version")

	applyServeFlags(appConfig, rootCmd)

	if err := loadEnvVars(appConfig); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.AddCommand(
		versionCommand(appConfig),
		serveCommand(appConfig),
	)

	// wanted behavior: shows an error when using the "help" subcommand and does not execute
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "",
		Hidden: true,
	})

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if commandHelpRequested(cmd) {
		os.Exit(0)
	}

	if appConfig.Verbose {
		fmt.Fprintln(os.Stderr, "Verbose mode enabled")
	}

	if appConfig.ShowVersion {
		fmt.Println(displayName + " version " + version + ", build " + commit)
		os.Exit(0)
	}

	if appConfig.Subcommand == "" {
		appConfig.Subcommand = "serve"
	}

	return appConfig
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
