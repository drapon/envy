package configure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureCommand(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("command_structure", func(t *testing.T) {
		assert.Equal(t, "configure", configureCmd.Use)
		assert.Equal(t, "Configure envy settings interactively", configureCmd.Short)
		assert.NotEmpty(t, configureCmd.Long)
		assert.NotEmpty(t, configureCmd.Example)
		assert.NotNil(t, configureCmd.RunE)
	})

	t.Run("flags", func(t *testing.T) {
		// Check profile flag
		profileFlag := configureCmd.Flag("profile")
		assert.NotNil(t, profileFlag)
		assert.Equal(t, "default", profileFlag.DefValue)

		// Check aws-region flag
		awsRegionFlag := configureCmd.Flag("aws-region")
		assert.NotNil(t, awsRegionFlag)

		// Check aws-profile flag
		awsProfileFlag := configureCmd.Flag("aws-profile")
		assert.NotNil(t, awsProfileFlag)

		// Check default-env flag
		defaultEnvFlag := configureCmd.Flag("default-env")
		assert.NotNil(t, defaultEnvFlag)

		// Check non-interactive flag
		nonInteractiveFlag := configureCmd.Flag("non-interactive")
		assert.NotNil(t, nonInteractiveFlag)
		assert.Equal(t, "bool", nonInteractiveFlag.Value.Type())
	})
}

func TestRunConfigure(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("non_interactive_mode", func(t *testing.T) {
		// Create a temporary directory for test
		helper := testutil.NewTestHelper(t)
		defer helper.Cleanup()
		
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)

		// Set flags for non-interactive mode
		nonInteractive = true
		awsRegion = "us-west-2"
		awsProfile = "test-profile"
		defaultEnv = "test"

		// Run configure
		cmd := &cobra.Command{}
		err := runConfigure(cmd, []string{})
		
		assert.NoError(t, err)

		// Verify configuration file was created
		testutil.AssertFileExists(t, ".envyrc")

		// Load and verify configuration
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		assert.Equal(t, "us-west-2", cfg.AWS.Region)
		assert.Equal(t, "test-profile", cfg.AWS.Profile)
		assert.Equal(t, "test", cfg.DefaultEnvironment)
	})

	t.Run("interactive_mode_skip", func(t *testing.T) {
		// For interactive mode, we'll skip the test as it requires user input
		// In a real scenario, we would mock the wizard.InteractiveInit function
		t.Skip("Skipping interactive mode test - requires user input")
	})
}

func TestConfigureNonInteractive(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("create_new_config", func(t *testing.T) {
		// Create a temporary directory for test
		helper := testutil.NewTestHelper(t)
		defer helper.Cleanup()
		
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)

		// Set test values
		awsRegion = "eu-west-1"
		awsProfile = "production"
		defaultEnv = "prod"

		// Run non-interactive configuration
		err := configureNonInteractive()
		assert.NoError(t, err)

		// Verify file was created
		testutil.AssertFileExists(t, ".envyrc")

		// Load and verify configuration
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		assert.Equal(t, "eu-west-1", cfg.AWS.Region)
		assert.Equal(t, "production", cfg.AWS.Profile)
		assert.Equal(t, "prod", cfg.DefaultEnvironment)
	})

	t.Run("update_existing_config", func(t *testing.T) {
		// Create a temporary directory for test
		helper := testutil.NewTestHelper(t)
		defer helper.Cleanup()
		
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)

		// Create an existing config file
		existingCfg := config.DefaultConfig()
		existingCfg.Project = "test-project"
		existingCfg.AWS.Region = "us-east-1"
		err := existingCfg.Save(".envyrc")
		require.NoError(t, err)

		// Set test values - only update region
		awsRegion = "ap-northeast-1"
		awsProfile = ""
		defaultEnv = ""

		// Run non-interactive configuration
		err = configureNonInteractive()
		assert.NoError(t, err)

		// Load and verify configuration
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		assert.Equal(t, "test-project", cfg.Project) // Should preserve existing value
		assert.Equal(t, "ap-northeast-1", cfg.AWS.Region) // Should be updated
		assert.Equal(t, "default", cfg.AWS.Profile) // Should use default
	})

	t.Run("invalid_configuration", func(t *testing.T) {
		// Create a temporary directory for test
		helper := testutil.NewTestHelper(t)
		defer helper.Cleanup()
		
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)

		// Create a config that will fail validation
		existingCfg := config.DefaultConfig()
		existingCfg.Project = "" // Invalid - empty project
		err := existingCfg.Save(".envyrc")
		require.NoError(t, err)

		// Don't set any flags
		awsRegion = ""
		awsProfile = ""
		defaultEnv = ""

		// Run non-interactive configuration
		err = configureNonInteractive()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid configuration")
	})

	t.Run("save_error", func(t *testing.T) {
		// Create a temporary directory for test
		helper := testutil.NewTestHelper(t)
		defer helper.Cleanup()
		
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)

		// Create a read-only directory
		readOnlyDir := filepath.Join(tempDir, "readonly")
		err := os.Mkdir(readOnlyDir, 0555)
		require.NoError(t, err)
		
		testutil.ChangeDir(t, readOnlyDir)

		// Set test values
		awsRegion = "us-west-2"
		awsProfile = ""
		defaultEnv = ""

		// Run non-interactive configuration
		err = configureNonInteractive()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save configuration")
	})
}

func TestConfigureIntegration(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("full_configuration_flow", func(t *testing.T) {
		// Create a temporary directory for test
		helper := testutil.NewTestHelper(t)
		defer helper.Cleanup()
		
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)

		// Reset flags
		profileName = "default"
		awsRegion = "us-west-2"
		awsProfile = "test-profile"
		defaultEnv = "development"
		nonInteractive = true

		// Create command as it would be in real usage
		cmd := &cobra.Command{}

		// Run the command
		err := runConfigure(cmd, []string{})
		assert.NoError(t, err)

		// Verify the configuration was saved correctly
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)

		// Verify all settings
		assert.Equal(t, "us-west-2", cfg.AWS.Region)
		assert.Equal(t, "test-profile", cfg.AWS.Profile)
		assert.Equal(t, "development", cfg.DefaultEnvironment)
		
		// Verify default values are present
		assert.NotEmpty(t, cfg.Project)
		assert.Len(t, cfg.Environments, 1) // Should have default environment
	})
}