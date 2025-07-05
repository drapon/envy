package init

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/color"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/wizard"
	"github.com/spf13/cobra"
)

var (
	projectName string
	envName     string
	awsRegion   string
	awsProfile  string
	interactive bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new envy project",
	Long: `Initialize a new envy project in the current directory.

This command creates a .envyrc configuration file with default settings
that you can customize for your project.`,
	Example: `  # Initialize with default settings
  envy init
  
  # Initialize with custom project name
  envy init --project myapp
  
  # Initialize with AWS settings
  envy init --project myapp --aws-region us-west-2 --aws-profile prod`,
	RunE: runInit,
}

func init() {
	root.GetRootCmd().AddCommand(initCmd)

	// Add flags specific to init command
	initCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (defaults to current directory name)")
	initCmd.Flags().StringVarP(&envName, "env", "e", "dev", "Initial environment name")
	initCmd.Flags().StringVar(&awsRegion, "aws-region", "us-east-1", "AWS region")
	initCmd.Flags().StringVar(&awsProfile, "aws-profile", "default", "AWS profile")
	initCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if running in interactive mode
	if interactive {
		return wizard.InteractiveInit(projectName)
	}

	// Check if .envyrc already exists
	if _, err := os.Stat(".envyrc"); err == nil {
		return fmt.Errorf(".envyrc file already exists in current directory")
	}

	// Get project name from flag or current directory
	if projectName == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		projectName = filepath.Base(cwd)
	}

	// Detect existing .env files
	existingEnvFiles := detectEnvFiles()
	if len(existingEnvFiles) == 0 {
		color.PrintInfof("No existing .env files found in current directory")
		color.PrintInfof("Creating default configuration for 'dev' environment")
	}

	// Create configuration
	cfg := &config.Config{
		Project:            projectName,
		DefaultEnvironment: envName,
		AWS: config.AWSConfig{
			Service: "parameter_store",
			Region:  awsRegion,
			Profile: awsProfile,
		},
		Environments: make(map[string]config.Environment),
	}

	// Configure environments based on existing files
	if len(existingEnvFiles) > 0 {
		color.PrintInfof("Found existing .env files: %v", existingEnvFiles)
		for _, envFile := range existingEnvFiles {
			envNameFromFile := extractEnvName(envFile)
			cfg.Environments[envNameFromFile] = config.Environment{
				Files: []string{envFile},
				Path:  fmt.Sprintf("/%s/%s/", projectName, envNameFromFile),
			}
			// Set first environment as default if not specified
			if cfg.DefaultEnvironment == "dev" && envNameFromFile != "dev" {
				cfg.DefaultEnvironment = envNameFromFile
			}
		}
	} else {
		// No existing files, create default dev environment
		cfg.Environments[envName] = config.Environment{
			Files: []string{fmt.Sprintf(".env.%s", envName)},
			Path:  fmt.Sprintf("/%s/%s/", projectName, envName),
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Save configuration
	if err := cfg.Save(".envyrc"); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Only create example .env file if no existing env files were found
	if len(existingEnvFiles) == 0 {
		envFile := fmt.Sprintf(".env.%s", envName)
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			content := `# Example environment variables
DATABASE_URL=postgresql://localhost/myapp_dev
REDIS_URL=redis://localhost:6379
API_KEY=your-api-key-here
DEBUG=true
`
			if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
				color.PrintWarningf("Failed to create example %s file: %v", envFile, err)
			} else {
				color.PrintSuccessf("Created example %s file", envFile)
			}
		}
	}

	color.PrintSuccessf("Successfully initialized envy project '%s'", projectName)
	color.PrintSuccessf("Created .envyrc configuration file")
	color.PrintBoldf("\nNext steps:")
	color.PrintInfof("1. Edit .envyrc to customize your configuration")
	if len(existingEnvFiles) > 0 {
		color.PrintInfof("2. Review the detected environment files in .envyrc")
	} else {
		color.PrintInfof("2. Create or edit .env.%s with your environment variables", envName)
	}
	color.PrintInfof("3. Run 'envy push' to sync variables to AWS")

	return nil
}

// detectEnvFiles scans the current directory for .env files
func detectEnvFiles() []string {
	var envFiles []string

	// Common .env file patterns
	patterns := []string{
		".env",
		".env.*",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, match := range matches {
				// Skip .env.example or .env.sample files
				if filepath.Ext(match) == ".example" || filepath.Ext(match) == ".sample" {
					continue
				}
				envFiles = append(envFiles, match)
			}
		}
	}

	return envFiles
}

// extractEnvName extracts environment name from .env file name
func extractEnvName(filename string) string {
	base := filepath.Base(filename)

	// Handle different naming patterns
	switch base {
	case ".env":
		return "default"
	case ".env.local":
		return "local"
	case ".env.production", ".env.prod":
		return "prod"
	case ".env.development", ".env.dev":
		return "dev"
	case ".env.staging", ".env.stage":
		return "staging"
	case ".env.test":
		return "test"
	default:
		// For .env.xxx pattern, extract xxx
		if len(base) > 5 && base[:5] == ".env." {
			return base[5:]
		}
		return "default"
	}
}
