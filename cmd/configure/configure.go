package configure

import (
	"fmt"
	"os"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/wizard"
	"github.com/spf13/cobra"
)

var (
	profileName    string
	awsRegion      string
	awsProfile     string
	defaultEnv     string
	nonInteractive bool
)

// configureCmd represents the configure command
var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure envy settings interactively",
	Long: `Configure envy settings through an interactive setup process.

This command guides you through setting up AWS credentials, default
environments, and other envy configuration options.`,
	Example: `  # Start interactive configuration
  envy configure
  
  # Configure AWS settings only
  envy configure aws
  
  # Configure a specific profile
  envy configure --profile production
  
  # Non-interactive configuration with flags
  envy configure --aws-region us-west-2 --aws-profile myprofile`,
	RunE: runConfigure,
}

func init() {
	root.GetRootCmd().AddCommand(configureCmd)
	
	// Add flags specific to configure command
	configureCmd.Flags().StringVarP(&profileName, "profile", "p", "default", "Configuration profile name")
	configureCmd.Flags().StringVar(&awsRegion, "aws-region", "", "AWS region")
	configureCmd.Flags().StringVar(&awsProfile, "aws-profile", "", "AWS profile name")
	configureCmd.Flags().StringVar(&defaultEnv, "default-env", "", "Default environment")
	configureCmd.Flags().BoolVarP(&nonInteractive, "non-interactive", "n", false, "Run in non-interactive mode")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	// Check if running in non-interactive mode
	if nonInteractive {
		return configureNonInteractive()
	}

	// Run interactive wizard
	return wizard.InteractiveInit("")
}

func configureNonInteractive() error {
	// Load existing config or create new one
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Apply flags if provided
	if awsRegion != "" {
		cfg.AWS.Region = awsRegion
	}
	if awsProfile != "" {
		cfg.AWS.Profile = awsProfile
	}
	if defaultEnv != "" {
		cfg.DefaultEnvironment = defaultEnv
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Save configuration
	configFile := ".envyrc"
	if _, err := os.Stat(configFile); err == nil {
		// Update existing file
		fmt.Printf("Updating %s...\n", configFile)
	} else {
		// Create new file
		fmt.Printf("Creating %s...\n", configFile)
	}

	if err := cfg.Save(configFile); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("Configuration saved successfully!")
	return nil
}