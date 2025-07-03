package root

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/drapon/envy/internal/cache"
	"github.com/drapon/envy/internal/color"
	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/updater"
	"github.com/drapon/envy/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	debug      bool
	verbose    bool
	quiet      bool
	noColor    bool
	noCache    bool
	clearCache bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "envy",
	Short: "A CLI tool for managing environment variables with AWS",
	Long: `envy is a CLI tool that simplifies environment variable management 
by syncing between local .env files and AWS Parameter Store/Secrets Manager.

It provides seamless bidirectional synchronization, multi-environment support,
and secure storage for your application's configuration.`,
	Version: version.GetInfo().Version,
	Example: `  # Initialize a new project
  envy init

  # Push environment variables to AWS
  envy push --env dev

  # Pull environment variables from AWS
  envy pull --env prod

  # List all environment variables
  envy list --env staging`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Ensure proper log flushing and cache closure
	defer func() {
		cache.CloseGlobalCache()
		log.FlushLogs()
	}()
	
	// Check for updates in background
	if !viper.GetBool("no_update_check") {
		updater.CheckAndNotify(rootCmd.Context(), version.GetInfo().Version)
	}
	
	if err := rootCmd.Execute(); err != nil {
		log.Error("Command execution error", log.ErrorField(err))
		fmt.Fprintln(os.Stderr, color.FormatError(err.Error()))
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Persistent flags - global for all commands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .envyrc in current directory)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "disable cache usage")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "clear cache before executing command")
	rootCmd.PersistentFlags().Bool("no-update-check", false, "disable automatic update check")

	// Bind flags to viper
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
	_ = viper.BindPFlag("no_cache", rootCmd.PersistentFlags().Lookup("no-cache"))
	_ = viper.BindPFlag("clear_cache", rootCmd.PersistentFlags().Lookup("clear-cache"))

	// Set custom version template
	rootCmd.SetVersionTemplate(version.GetInfo().DetailedString())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find current working directory.
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, color.FormatError(err.Error()))
			os.Exit(1)
		}

		// Search config in current directory with name ".envyrc" (without extension).
		viper.AddConfigPath(cwd)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".envyrc")

		// Also check parent directories for .envyrc
		dir := cwd
		for dir != "/" && dir != "." {
			viper.AddConfigPath(dir)
			dir = filepath.Dir(dir)
		}
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("ENVY")
	viper.AutomaticEnv() // read in environment variables that match

	// Set defaults
	viper.SetDefault("project", "myapp")
	viper.SetDefault("default_environment", "dev")
	viper.SetDefault("aws.service", "parameter_store")
	viper.SetDefault("aws.region", "us-east-1")
	viper.SetDefault("aws.profile", "default")
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.ttl", "1h")
	viper.SetDefault("cache.max_size", "100MB")
	viper.SetDefault("cache.max_entries", 1000)
	viper.SetDefault("update.check_enabled", true)
	viper.SetDefault("update.check_interval", "24h")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if debug || verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
		log.LogConfigLoad(viper.ConfigFileUsed(), true, nil)
	} else {
		// It's not an error if the config file is not found
		log.LogConfigLoad("", false, err)
	}
	
	// Initialize logging system
	if err := log.InitializeLogger(viper.GetViper()); err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", color.FormatError("Failed to initialize logger:"), err)
		// Continue program even if logger initialization fails
	}
	
	// Initialize cache system
	if err := cache.InitGlobalCache(viper.GetViper()); err != nil {
		log.Warn("Failed to initialize cache system", log.ErrorField(err))
		// Continue program even if cache initialization fails (will work without cache)
	}
}

// GetRootCmd returns the root command
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// IsDebug returns true if debug mode is enabled
func IsDebug() bool {
	return viper.GetBool("debug")
}

// IsVerbose returns true if verbose mode is enabled
func IsVerbose() bool {
	return viper.GetBool("verbose")
}

// IsNoColor returns true if colored output is disabled
func IsNoColor() bool {
	return viper.GetBool("no_color")
}

// IsNoCache returns true if cache usage is disabled
func IsNoCache() bool {
	return viper.GetBool("no_cache")
}

// IsClearCache returns true if cache should be cleared
func IsClearCache() bool {
	return viper.GetBool("clear_cache")
}

// AddCommand adds a command to the root command
func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}