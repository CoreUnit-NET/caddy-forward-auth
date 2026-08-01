package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const serviceEnvPrefix = "SERVICE_"

type ServiceCred struct {
	HostGlob     string
	Username     string
	PasswordHash string
}

type AppConfig struct {
	Verbose     bool
	ShowVersion bool
	Args        []string
	Subcommand  string

	Host           string
	Port           int
	AllowedOrigins string
	Services       map[string]ServiceCred
}

func defaultAppConfig() *AppConfig {
	return &AppConfig{
		Verbose:     false,
		ShowVersion: false,
		Args:        []string{},

		Host:           "0.0.0.0",
		Port:           8080,
		AllowedOrigins: "",
		Services:       map[string]ServiceCred{},
	}
}

func versionCommand(appConfig *AppConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints version message",
		Run: func(cmd *cobra.Command, args []string) {
			appConfig.Args = args
			appConfig.ShowVersion = true
		},
	}
}

func serveCommand(appConfig *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Run: func(cmd *cobra.Command, args []string) {
			appConfig.Args = args
			appConfig.Subcommand = "serve"
		},
	}
	applyServeFlags(appConfig, cmd)
	return cmd
}

func loadEnvVars(appConfig *AppConfig) {
	EnvIsBool("VERBOSE", func(value bool) {
		appConfig.Verbose = value
	})
	EnvIsString("HOST", func(value string) {
		appConfig.Host = value
	})
	EnvIsInt("PORT", func(value int) {
		appConfig.Port = value
	})
	EnvIsString("ALLOWED_ORIGINS", func(value string) {
		appConfig.AllowedOrigins = value
	})
}

// loadServiceEnvVars scans the process environment for SERVICE_* entries.
// Values must be hostGlob/username/passwordHash with exactly two separating slashes
// (password hashes may contain additional '/' characters).
func loadServiceEnvVars(appConfig *AppConfig) error {
	services := make(map[string]ServiceCred)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(key, serviceEnvPrefix) {
			continue
		}
		name := strings.TrimPrefix(key, serviceEnvPrefix)
		if name == "" {
			return fmt.Errorf("invalid service env %q: empty service name", key)
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		cred, err := parseServiceValue(key, value)
		if err != nil {
			return err
		}
		services[name] = cred
	}
	appConfig.Services = services
	return nil
}

func parseServiceValue(envKey, value string) (ServiceCred, error) {
	parts := strings.SplitN(value, "/", 3)
	if len(parts) != 3 {
		return ServiceCred{}, fmt.Errorf(
			"invalid %s value %q: want hostGlob/username/passwordHash (exactly 2 separating slashes)",
			envKey,
			value,
		)
	}
	hostGlob := strings.TrimSpace(parts[0])
	username := strings.TrimSpace(parts[1])
	passwordHash := strings.TrimSpace(parts[2])
	if hostGlob == "" || username == "" || passwordHash == "" {
		return ServiceCred{}, fmt.Errorf(
			"invalid %s value %q: hostGlob, username, and passwordHash must be non-empty",
			envKey,
			value,
		)
	}
	return ServiceCred{
		HostGlob:     hostGlob,
		Username:     username,
		PasswordHash: passwordHash,
	}, nil
}

func applyServeFlags(appConfig *AppConfig, cmd *cobra.Command) {
	cmd.Flags().StringVar(&appConfig.Host, "host", appConfig.Host, "bind address (HOST)")
	cmd.Flags().IntVar(&appConfig.Port, "port", appConfig.Port, "listen port (PORT)")
	cmd.Flags().StringVar(&appConfig.AllowedOrigins, "allowed-origins", appConfig.AllowedOrigins, "CSV of allowed Origin hostnames (ALLOWED_ORIGINS)")
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
			appConfig.Args = args
			appConfig.Subcommand = "serve"
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&appConfig.Verbose, "verbose", "b", appConfig.Verbose, "enable verbose mode (VERBOSE)")
	rootCmd.Flags().BoolVarP(&appConfig.ShowVersion, "version", "v", appConfig.ShowVersion, "prints version")

	// Serve flags on root so bare `intern-auth-gateway --port 8080` works.
	applyServeFlags(appConfig, rootCmd)

	loadEnvVars(appConfig)

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

	if err := loadServiceEnvVars(appConfig); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
