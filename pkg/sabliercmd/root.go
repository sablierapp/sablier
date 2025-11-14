package sabliercmd

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	// The name of our config file, without the file extension because viper supports many different config file languages.
	defaultConfigFilename = "sablier"
)

var conf = config.NewConfig()
var cfgFile string

// NewRootCommand creates the root cobra command
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sablier",
		Short: "A webserver to start container on demand",
		Long: `Sablier is an API that starts containers on demand.
It provides integrations with multiple reverse proxies and different loading strategies.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
			return initializeConfig(cmd)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "configFile", "", "Config file path. If not defined, looks for sablier.(yml|yaml|toml) in /etc/sablier/ > $XDG_CONFIG_HOME > $HOME/.config/ and current directory")

	startCmd := newStartCommand()
	// Provider flags
	startCmd.Flags().StringVar(&conf.Provider.Name, "provider.name", "docker", fmt.Sprintf("Provider to use to manage containers %v", config.GetProviders()))
	_ = viper.BindPFlag("provider.name", startCmd.Flags().Lookup("provider.name"))
	startCmd.Flags().BoolVar(&conf.Provider.AutoStopOnStartup, "provider.auto-stop-on-startup", true, "")
	_ = viper.BindPFlag("provider.auto-stop-on-startup", startCmd.Flags().Lookup("provider.auto-stop-on-startup"))
	startCmd.Flags().Float32Var(&conf.Provider.Kubernetes.QPS, "provider.kubernetes.qps", 5, "QPS limit for K8S API access client-side throttling")
	_ = viper.BindPFlag("provider.kubernetes.qps", startCmd.Flags().Lookup("provider.kubernetes.qps"))
	startCmd.Flags().IntVar(&conf.Provider.Kubernetes.Burst, "provider.kubernetes.burst", 10, "Maximum burst for K8S API acees client-side throttling")
	_ = viper.BindPFlag("provider.kubernetes.burst", startCmd.Flags().Lookup("provider.kubernetes.burst"))
	startCmd.Flags().StringVar(&conf.Provider.Kubernetes.Delimiter, "provider.kubernetes.delimiter", "_", "Delimiter used for namespace/resource type/name resolution. Defaults to \"_\" for backward compatibility. But you should use \"/\" or \".\"")
	_ = viper.BindPFlag("provider.kubernetes.delimiter", startCmd.Flags().Lookup("provider.kubernetes.delimiter"))
	startCmd.Flags().StringVar(&conf.Provider.Podman.Uri, "provider.podman.uri", "unix:///run/podman/podman.sock", "Uri is the URI to connect to the Podman service.")
	_ = viper.BindPFlag("provider.podman.uri", startCmd.Flags().Lookup("provider.podman.uri"))

	// Server flags
	startCmd.Flags().IntVar(&conf.Server.Port, "server.port", 10000, "The server port to use")
	_ = viper.BindPFlag("server.port", startCmd.Flags().Lookup("server.port"))
	startCmd.Flags().StringVar(&conf.Server.BasePath, "server.base-path", "/", "The base path for the API")
	_ = viper.BindPFlag("server.base-path", startCmd.Flags().Lookup("server.base-path"))
	// Storage flags
	startCmd.Flags().StringVar(&conf.Storage.File, "storage.file", "", "File path to save the state")
	_ = viper.BindPFlag("storage.file", startCmd.Flags().Lookup("storage.file"))
	// Sessions flags
	startCmd.Flags().DurationVar(&conf.Sessions.DefaultDuration, "sessions.default-duration", time.Duration(5)*time.Minute, "The default session duration")
	_ = viper.BindPFlag("sessions.default-duration", startCmd.Flags().Lookup("sessions.default-duration"))
	startCmd.Flags().DurationVar(&conf.Sessions.ExpirationInterval, "sessions.expiration-interval", time.Duration(20)*time.Second, "The expiration checking interval. Higher duration gives less stress on CPU. If you only use sessions of 1h, setting this to 5m is a good trade-off.")
	_ = viper.BindPFlag("sessions.expiration-interval", startCmd.Flags().Lookup("sessions.expiration-interval"))

	// logging level
	rootCmd.PersistentFlags().StringVar(&conf.Logging.Level, "logging.level", strings.ToLower(slog.LevelInfo.String()), "The logging level. Can be one of [error, warn, info, debug]")
	_ = viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("logging.level"))

	// strategy
	startCmd.Flags().StringVar(&conf.Strategy.Dynamic.CustomThemesPath, "strategy.dynamic.custom-themes-path", "", "Custom themes folder, will load all .html files recursively")
	_ = viper.BindPFlag("strategy.dynamic.custom-themes-path", startCmd.Flags().Lookup("strategy.dynamic.custom-themes-path"))
	startCmd.Flags().StringVar(&conf.Strategy.Dynamic.DefaultTheme, "strategy.dynamic.default-theme", "hacker-terminal", "Default theme used for dynamic strategy")
	_ = viper.BindPFlag("strategy.dynamic.default-theme", startCmd.Flags().Lookup("strategy.dynamic.default-theme"))
	startCmd.Flags().BoolVar(&conf.Strategy.Dynamic.ShowDetailsByDefault, "strategy.dynamic.show-details-by-default", true, "Show the loading instances details by default")
	_ = viper.BindPFlag("strategy.dynamic.show-details-by-default", startCmd.Flags().Lookup("strategy.dynamic.show-details-by-default"))
	startCmd.Flags().DurationVar(&conf.Strategy.Dynamic.DefaultRefreshFrequency, "strategy.dynamic.default-refresh-frequency", 5*time.Second, "Default refresh frequency in the HTML page for dynamic strategy")
	_ = viper.BindPFlag("strategy.dynamic.default-refresh-frequency", startCmd.Flags().Lookup("strategy.dynamic.default-refresh-frequency"))
	startCmd.Flags().DurationVar(&conf.Strategy.Blocking.DefaultTimeout, "strategy.blocking.default-timeout", 1*time.Minute, "Default timeout used for blocking strategy")
	_ = viper.BindPFlag("strategy.blocking.default-timeout", startCmd.Flags().Lookup("strategy.blocking.default-timeout"))
	startCmd.Flags().DurationVar(&conf.Strategy.Blocking.DefaultRefreshFrequency, "strategy.blocking.default-refresh-frequency", 5*time.Second, "Default refresh frequency at which the instances status are checked for blocking strategy")
	_ = viper.BindPFlag("strategy.blocking.default-refresh-frequency", startCmd.Flags().Lookup("strategy.blocking.default-refresh-frequency"))

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(NewVersionCmd())

	healthCmd := NewHealthCmd()
	healthCmd.Flags().String("url", "http://localhost:10000/health", "Sablier health endpoint")
	rootCmd.AddCommand(healthCmd)

	return rootCmd
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()

	// Set the base name of the config file, without the file extension.
	v.SetConfigName(defaultConfigFilename)

	v.AddConfigPath("/etc/sablier/")
	v.AddConfigPath("$XDG_CONFIG_HOME")
	v.AddConfigPath("$HOME/.config/")
	v.AddConfigPath(".")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}

	// Attempt to read the config file, gracefully ignoring errors
	// caused by a config file not being found. Return an error
	// if we cannot parse the config file.
	if err := v.ReadInConfig(); err != nil {
		// It's okay if there isn't a config file
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return err
		}
	}

	// Bind to environment variables
	// Works great for simple config names, but needs help for names
	// like --favorite-color which we fix in the bindFlags function
	v.AutomaticEnv()

	// Bind the current command's flags to viper
	bindFlags(cmd, v)

	return nil
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		envVarSuffix = strings.ToUpper(strings.ReplaceAll(envVarSuffix, ".", "_"))
		_ = v.BindEnv(f.Name, envVarSuffix)

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
