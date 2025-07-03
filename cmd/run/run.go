package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	environment string
	envFiles    []string
	setVars     []string
	override    bool
	inherit     bool
	dryRun      bool
	verbose     bool
	from        string
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run -- [command]",
	Short: "Run a command with environment variables",
	Long: `Run a command with environment variables loaded from envy.

This command loads environment variables from your configured sources
and executes the specified command with those variables available.`,
	Example: `  # Run a command with loaded env vars
  envy run -- npm start
  
  # Run with a specific environment
  envy run --env production -- python app.py
  
  # Run with additional env files
  envy run --file .env.common --file .env.dev -- make test
  
  # Run with temporary variables
  envy run --set DEBUG=true --set PORT=3000 -- node server.js
  
  # Run with AWS parameters
  envy run --env production --from aws -- ./deploy.sh
  
  # Dry run to see what would be executed
  envy run --dry-run -- npm start`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCommand,
}

func init() {
	root.GetRootCmd().AddCommand(runCmd)

	// Add flags specific to run command
	runCmd.Flags().StringVarP(&environment, "env", "e", "", "Environment to use")
	runCmd.Flags().StringSliceVarP(&envFiles, "file", "f", []string{}, "Additional .env files to load")
	runCmd.Flags().StringSliceVarP(&setVars, "set", "s", []string{}, "Set environment variables (KEY=VALUE format)")
	runCmd.Flags().BoolVarP(&override, "override", "o", false, "Override existing environment variables")
	runCmd.Flags().BoolVarP(&inherit, "inherit", "i", true, "Inherit current process environment variables")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show command and environment without executing")
	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose output")
	runCmd.Flags().StringVar(&from, "from", "local", "Source of variables (local/aws)")
}

func runCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Build environment variables
	envVars, err := buildEnvironment(ctx)
	if err != nil {
		return fmt.Errorf("failed to build environment: %w", err)
	}

	// Handle dry run
	if dryRun {
		return showDryRun(args, envVars)
	}

	// Execute command
	return executeCommand(args, envVars)
}

func buildEnvironment(ctx context.Context) ([]string, error) {
	// Create environment manager
	envManager := env.NewManager(".")

	// Start with current environment if inherit is true
	envMap := make(map[string]string)
	if inherit {
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
		if verbose {
			fmt.Printf("Inherited %d environment variables from current process\n", len(envMap))
		}
	}

	// Load configuration
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Could not load configuration: %v\n", err)
		}
		// Continue without config for local mode
		if from == "aws" {
			return nil, fmt.Errorf("configuration required for AWS mode: %w", err)
		}
	}

	// Load environment variables based on source
	if from == "aws" && cfg != nil {
		if err := loadFromAWS(ctx, cfg, envMap); err != nil {
			return nil, err
		}
	} else {
		if err := loadFromLocal(cfg, envManager, envMap); err != nil {
			return nil, err
		}
	}

	// Load additional env files
	for _, file := range envFiles {
		if verbose {
			fmt.Printf("Loading additional file: %s\n", file)
		}
		envFile, err := env.ParseFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to load file %s: %w", file, err)
		}
		applyEnvFile(envFile, envMap)
	}

	// Apply command-line set variables
	for _, v := range setVars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid variable format: %s (expected KEY=VALUE)", v)
		}
		key, value := parts[0], parts[1]
		if _, exists := envMap[key]; exists && !override {
			if verbose {
				fmt.Printf("Skipping %s (already set, use --override to force)\n", key)
			}
		} else {
			envMap[key] = value
			if verbose {
				fmt.Printf("Set %s\n", key)
			}
		}
	}

	// Convert map to slice
	var envVars []string
	for k, v := range envMap {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	if verbose {
		fmt.Printf("Total environment variables: %d\n", len(envVars))
	}

	return envVars, nil
}

func loadFromAWS(ctx context.Context, cfg *config.Config, envMap map[string]string) error {
	// Create AWS manager
	awsManager, err := aws.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create AWS manager: %w", err)
	}

	// Use environment from flag or default
	envName := environment
	if envName == "" {
		envName = cfg.DefaultEnvironment
	}

	if verbose {
		fmt.Printf("Loading environment '%s' from AWS...\n", envName)
	}

	// Pull environment from AWS
	envFile, err := awsManager.PullEnvironment(ctx, envName)
	if err != nil {
		return fmt.Errorf("failed to pull from AWS: %w", err)
	}

	applyEnvFile(envFile, envMap)
	if verbose {
		fmt.Printf("Loaded %d variables from AWS\n", len(envFile.Variables))
	}

	return nil
}

func loadFromLocal(cfg *config.Config, envManager *env.Manager, envMap map[string]string) error {
	var filesToLoad []string

	// If additional files are specified via --file flag, skip config-based loading
	if len(envFiles) == 0 {
		if cfg != nil && environment != "" {
			// Load environment-specific files from config
			envConfig, err := cfg.GetEnvironment(environment)
			if err != nil {
				return err
			}
			filesToLoad = envConfig.Files
		} else if cfg != nil && cfg.DefaultEnvironment != "" {
			// Load default environment files
			envConfig, err := cfg.GetEnvironment(cfg.DefaultEnvironment)
			if err == nil {
				filesToLoad = envConfig.Files
			}
		} else {
			// Default to .env if no config
			filesToLoad = []string{".env"}
		}

		// Load each file
		for _, file := range filesToLoad {
			if verbose {
				fmt.Printf("Loading file: %s\n", file)
			}
			envFile, err := env.ParseFile(file)
			if err != nil {
				if os.IsNotExist(err) {
					if verbose {
						fmt.Printf("File not found: %s\n", file)
					}
					continue
				}
				return fmt.Errorf("failed to load file %s: %w", file, err)
			}
			applyEnvFile(envFile, envMap)
		}
	}

	return nil
}

func applyEnvFile(envFile *env.File, envMap map[string]string) {
	for key, variable := range envFile.Variables {
		if _, exists := envMap[key]; exists && !override {
			if verbose {
				fmt.Printf("Skipping %s (already set, use --override to force)\n", key)
			}
		} else {
			envMap[key] = variable.Value
		}
	}
}

func showDryRun(args []string, envVars []string) error {
	fmt.Println("DRY RUN MODE - Command will not be executed")
	fmt.Println()
	fmt.Printf("Command: %s\n", strings.Join(args, " "))
	fmt.Println()
	fmt.Println("Environment variables:")
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key, value := parts[0], parts[1]
			// Mask sensitive values in dry run
			if isSensitive(key) {
				value = maskValue(value)
			}
			fmt.Printf("  %s=%s\n", key, value)
		}
	}
	return nil
}

func executeCommand(args []string, envVars []string) error {
	// Get the command and its arguments
	cmdName := args[0]
	cmdArgs := args[1:]

	// Create command
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Env = envVars
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// TODO: Implement platform-specific process management
	// For now, commenting out Unix-specific code for Windows compatibility
	
	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		if cmd.Process != nil {
			// Use cross-platform signal
			cmd.Process.Signal(os.Interrupt)
		}
	}()

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Wait for command to complete
	err := cmd.Wait()

	// Handle exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Command exited with non-zero status
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

func isSensitive(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "passwd", "pwd",
		"secret",
		"key", "api_key", "apikey",
		"token",
		"auth",
		"credential",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}
	return false
}

func maskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}
