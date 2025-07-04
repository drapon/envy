package pull

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/cache"
	"github.com/drapon/envy/internal/color"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/log"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	environment string
	prefix      string
	output      string
	export      bool
	overwrite   bool
	all         bool
	backup      bool
	merge       bool
	noProgress  bool
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull environment variables from AWS",
	Long: `Pull environment variables from AWS Parameter Store or Secrets Manager.

This command downloads variables from AWS and saves them to local .env files
based on your configuration in .envyrc.`,
	Example: `  # Pull variables for the default environment
  envy pull
  
  # Pull variables for a specific environment
  envy pull --env production
  
  # Pull and export to shell
  envy pull --export
  
  # Pull to a specific file
  envy pull --output .env.prod
  
  # Pull all environments
  envy pull --all
  
  # Pull with backup of existing files
  envy pull --backup`,
	RunE: runPull,
}

func init() {
	root.GetRootCmd().AddCommand(pullCmd)

	// Add flags specific to pull command
	pullCmd.Flags().StringVarP(&environment, "env", "e", "", "Source environment")
	pullCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "AWS parameter prefix (overrides config)")
	pullCmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (overrides config)")
	pullCmd.Flags().BoolVarP(&export, "export", "x", false, "Export variables to shell")
	pullCmd.Flags().BoolVarP(&overwrite, "overwrite", "w", false, "Overwrite existing file without backup")
	pullCmd.Flags().BoolVarP(&all, "all", "a", false, "Pull all environments")
	pullCmd.Flags().BoolVar(&backup, "backup", false, "Create backup of existing files")
	pullCmd.Flags().BoolVarP(&merge, "merge", "m", false, "Merge with existing local variables")
	pullCmd.Flags().BoolVar(&noProgress, "no-progress", false, "Disable progress bar")
}

func runPull(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := log.WithContext(zap.String("command", "pull"))

	// Load configuration with caching
	cfg, err := loadConfigWithCache()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create AWS manager
	awsManager, err := aws.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create AWS manager: %w", err)
	}

	// Determine which environments to pull
	environments := []string{}
	if all {
		for envName := range cfg.Environments {
			environments = append(environments, envName)
		}
	} else {
		if environment == "" {
			environment = cfg.DefaultEnvironment
		}
		environments = []string{environment}
	}

	// Process each environment
	for _, envName := range environments {
		if err := pullEnvironment(ctx, cfg, awsManager, envName, logger); err != nil {
			return fmt.Errorf("failed to pull environment %s: %w", envName, err)
		}
	}

	return nil
}

func pullEnvironment(ctx context.Context, cfg *config.Config, awsManager *aws.Manager, envName string, logger *zap.Logger) error {
	color.PrintInfo("Pulling environment: %s", envName)

	// Get environment configuration
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Pull from AWS with caching
	if !viper.GetBool("quiet") && !export && !noProgress {
		color.PrintInfo("Connecting to %s...", getSourceDescription(cfg, envName))
	}

	envFile, err := pullEnvironmentWithCache(ctx, awsManager, envName, logger)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	variableCount := len(envFile.Keys())
	if variableCount == 0 {
		color.PrintWarning("No variables found")
		return nil
	}

	color.PrintInfo("Fetched %d variables", variableCount)

	// Handle export mode
	if export {
		return exportVariables(envFile)
	}

	// Determine output file
	outputFile := output
	if outputFile == "" && len(envConfig.Files) > 0 {
		outputFile = envConfig.Files[0]
	}
	if outputFile == "" {
		outputFile = fmt.Sprintf(".env.%s", envName)
	}

	// Handle merge mode
	if merge && fileExists(outputFile) {
		existingFile, err := env.ParseFile(outputFile)
		if err != nil {
			color.PrintWarning("Could not parse existing file for merge: %v", err)
		} else {
			color.PrintInfo("Merging with existing %s...", outputFile)
			existingFile.Merge(envFile)
			envFile = existingFile
		}
	}

	// Create backup if file exists
	if backup && !overwrite && fileExists(outputFile) {
		backupFile := createBackupFilename(outputFile)
		color.PrintInfo("Creating backup: %s", backupFile)
		if err := copyFile(outputFile, backupFile); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Save to file with progress indication
	if !viper.GetBool("quiet") && !export {
		color.PrintInfo("Writing %d variables to %s...", variableCount, outputFile)
	}

	// Write the file
	if err := envFile.WriteFile(outputFile); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if !viper.GetBool("quiet") && !export {
		color.PrintSuccess("✓ File written successfully")
	}

	// Set file permissions to 600
	if err := os.Chmod(outputFile, 0600); err != nil {
		color.PrintWarning("Could not set file permissions: %v", err)
	}

	if !viper.GetBool("quiet") {
		color.PrintSuccess("Successfully pulled %d variables to %s", variableCount, outputFile)
	}
	return nil
}

func exportVariables(envFile *env.File) error {
	color.PrintInfo("\n# Export environment variables")
	color.PrintInfo("# Run: eval $(envy pull --export)")
	fmt.Println()

	for _, key := range envFile.SortedKeys() {
		value, _ := envFile.Get(key)
		// Escape single quotes in value
		escapedValue := strings.ReplaceAll(value, "'", "'\"'\"'")
		fmt.Printf("export %s='%s'\n", key, escapedValue)
	}

	return nil
}

func getSourceDescription(cfg *config.Config, envName string) string {
	service := cfg.GetAWSService(envName)
	path := cfg.GetParameterPath(envName)
	region := cfg.AWS.Region

	if service == "secrets_manager" {
		return fmt.Sprintf("AWS Secrets Manager (%s)", region)
	}

	return fmt.Sprintf("AWS Parameter Store %s (%s)", path, region)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func createBackupFilename(original string) string {
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(original, ext)
	timestamp := time.Now().Format("20060102_150405.000")
	return fmt.Sprintf("%s.backup_%s%s", base, timestamp, ext)
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, input, 0600)
	if err != nil {
		return err
	}

	return nil
}

// loadConfigWithCache loads configuration with cache support
func loadConfigWithCache() (*config.Config, error) {
	// Due to complex type issues with config file caching,
	// we only check file changes and reload each time
	configFile := viper.GetString("config")

	// Check file modification time
	if configFile != "" {
		if stat, err := os.Stat(configFile); err == nil {
			log.Debug("Loading configuration file",
				zap.String("file", configFile),
				zap.Time("modified", stat.ModTime()))
		}
	} else {
		// Search for default configuration file
		cwd, _ := os.Getwd()
		defaultConfigPath := filepath.Join(cwd, ".envyrc")
		if stat, err := os.Stat(defaultConfigPath); err == nil {
			log.Debug("Loading default configuration file",
				zap.String("file", defaultConfigPath),
				zap.Time("modified", stat.ModTime()))
		}
	}

	// Load configuration (without cache)
	return config.Load(configFile)
}

// pullEnvironmentWithCache retrieves environment variables from AWS with cache support
func pullEnvironmentWithCache(ctx context.Context, awsManager *aws.Manager, envName string, logger *zap.Logger) (*env.File, error) {
	// Generate cache key
	cacheKey := cache.NewCacheKeyBuilder("aws_env").
		Add(envName).
		Add(awsManager.GetConfig().AWS.Region).
		Add(awsManager.GetConfig().GetParameterPath(envName)).
		Build()

	// Get or generate environment variables from cache
	result, err := cache.CachedOperationWithMetadata(
		cacheKey,
		15*time.Minute, // AWS environment variables cache TTL
		map[string]interface{}{
			"type":        "aws_environment",
			"environment": envName,
			"sensitive":   true, // AWS data is subject to encryption
		},
		func() (interface{}, error) {
			logger.Debug("Fetching AWS environment variables (cache miss)",
				zap.String("environment", envName))

			// Check if we should show progress
			showProgress := !viper.GetBool("quiet") && !export && !noProgress

			if showProgress {
				// Use our custom progress function with English messages
				return pullWithProgress(ctx, awsManager, envName)
			}

			// When progress is disabled, use regular pull
			return awsManager.PullEnvironment(ctx, envName)
		},
	)

	if err != nil {
		return nil, err
	}

	envFile, ok := result.(*env.File)
	if !ok {
		return nil, fmt.Errorf("invalid cached environment file type")
	}

	logger.Debug("Retrieved AWS environment variables",
		zap.String("environment", envName),
		zap.Int("variable_count", len(envFile.Keys())))

	return envFile, nil
}

// pullWithProgress pulls environment variables with a progress bar
func pullWithProgress(ctx context.Context, awsManager *aws.Manager, envName string) (*env.File, error) {
	cfg := awsManager.GetConfig()
	service := cfg.GetAWSService(envName)
	path := cfg.GetParameterPath(envName)

	// For Secrets Manager, use regular pull (single operation)
	if service == "secrets_manager" {
		return awsManager.PullEnvironment(ctx, envName)
	}

	// For Parameter Store, first get the count of parameters
	paramStore := awsManager.GetParameterStore()
	parameters, err := paramStore.GetParametersByPath(ctx, path, false, false)
	if err != nil {
		// If we can't get the count, fall back to regular pull
		return awsManager.PullEnvironment(ctx, envName)
	}

	if len(parameters) == 0 {
		// No parameters to pull
		return env.NewFile(), nil
	}

	// Create progress bar
	bar := progressbar.NewOptions(len(parameters),
		progressbar.OptionSetDescription("Fetching variables from AWS"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("vars"),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]█[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)

	// Pull each parameter
	envFile := env.NewFile()
	for _, param := range parameters {
		// Get parameter value with decryption
		fullParam, err := paramStore.GetParameter(ctx, param.Name, true)
		if err != nil {
			bar.Add(1)
			continue // Skip failed parameters
		}

		// Extract key from path
		key := strings.TrimPrefix(fullParam.Name, path)
		key = strings.TrimPrefix(key, "/")

		envFile.Set(key, fullParam.Value)
		bar.Add(1)
	}

	bar.Finish()
	return envFile, nil
}
