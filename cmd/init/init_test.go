package init

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	// Get the init command from root
	rootCmd := root.GetRootCmd()
	initCommand, _, err := rootCmd.Find([]string{"init"})
	require.NoError(t, err)
	require.NotNil(t, initCommand)
	
	assert.Equal(t, "init", initCommand.Use)
	assert.Contains(t, initCommand.Short, "Initialize a new envy project")
	assert.NotEmpty(t, initCommand.Long)
	assert.NotEmpty(t, initCommand.Example)
	
	// Check flags
	assert.NotNil(t, initCommand.Flags().Lookup("project"))
	assert.NotNil(t, initCommand.Flags().Lookup("env"))
	assert.NotNil(t, initCommand.Flags().Lookup("aws-region"))
	assert.NotNil(t, initCommand.Flags().Lookup("aws-profile"))
	assert.NotNil(t, initCommand.Flags().Lookup("interactive"))
}

func TestRunInit(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	t.Run("default_initialization", func(t *testing.T) {
		// Change to temp directory
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset flags
		resetFlags()
		
		// Run init command
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Check .envyrc was created
		testutil.AssertFileExists(t, ".envyrc")
		
		// Load and verify config
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		assert.Equal(t, filepath.Base(tempDir), cfg.Project)
		assert.Equal(t, "dev", cfg.DefaultEnvironment)
		assert.Equal(t, "us-east-1", cfg.AWS.Region)
		assert.Equal(t, "default", cfg.AWS.Profile)
		assert.Equal(t, "parameter_store", cfg.AWS.Service)
		
		// Check environment config
		devEnv, exists := cfg.Environments["dev"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env.dev"}, devEnv.Files)
		assert.Equal(t, fmt.Sprintf("/%s/dev/", cfg.Project), devEnv.Path)
		
		// Check .env.dev was created (only when no existing files)
		if _, err := os.Stat(".env.dev"); err == nil {
			content := testutil.ReadFile(t, ".env.dev")
			assert.Contains(t, content, "DATABASE_URL=")
			assert.Contains(t, content, "API_KEY=")
			assert.Contains(t, content, "DEBUG=true")
		}
	})
	
	t.Run("custom_project_name", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset and set flags
		resetFlags()
		projectName = "my-awesome-app"
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		assert.Equal(t, "my-awesome-app", cfg.Project)
		assert.Equal(t, "/my-awesome-app/dev/", cfg.Environments["dev"].Path)
	})
	
	t.Run("custom_environment", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset and set flags
		resetFlags()
		envName = "staging"
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		assert.Equal(t, "staging", cfg.DefaultEnvironment)
		
		// Check environment config
		stagingEnv, exists := cfg.Environments["staging"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env.staging"}, stagingEnv.Files)
		
		// Check .env.staging was created (only when no existing files)
		if _, err := os.Stat(".env.staging"); err == nil {
			testutil.AssertFileExists(t, ".env.staging")
		}
	})
	
	t.Run("custom_aws_settings", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset and set flags
		resetFlags()
		awsRegion = "eu-west-1"
		awsProfile = "production"
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		assert.Equal(t, "eu-west-1", cfg.AWS.Region)
		assert.Equal(t, "production", cfg.AWS.Profile)
	})
	
	t.Run("all_custom_flags", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset and set all flags
		resetFlags()
		projectName = "test-project"
		envName = "test"
		awsRegion = "ap-southeast-1"
		awsProfile = "test-profile"
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		assert.Equal(t, "test-project", cfg.Project)
		assert.Equal(t, "test", cfg.DefaultEnvironment)
		assert.Equal(t, "ap-southeast-1", cfg.AWS.Region)
		assert.Equal(t, "test-profile", cfg.AWS.Profile)
		
		// Check environment config
		testEnv, exists := cfg.Environments["test"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env.test"}, testEnv.Files)
		assert.Equal(t, "/test-project/test/", testEnv.Path)
		
		// Check .env.test was created (only when no existing files)
		if _, err := os.Stat(".env.test"); err == nil {
			testutil.AssertFileExists(t, ".env.test")
		}
	})
	
	t.Run("existing_envyrc", func(t *testing.T) {
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Create existing .envyrc
		helper.CreateTempFile(".envyrc", "existing: config")
		
		// Reset flags
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ".envyrc file already exists")
	})
	
	t.Run("existing_env_file", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Create existing .env.dev
		existingContent := "EXISTING_VAR=value\n"
		testutil.WriteFile(t, tempDir, ".env.dev", existingContent)
		
		// Reset flags
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Check that .env.dev still contains original content
		content := testutil.ReadFile(t, ".env.dev")
		assert.Equal(t, existingContent, content)
	})
	
	t.Run("config_validation", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset and set invalid project name
		resetFlags()
		projectName = "" // This will use directory name
		
		// Create a directory with invalid name
		invalidDir := filepath.Join(tempDir, "")
		os.Mkdir(invalidDir, 0755)
		testutil.ChangeDir(t, tempDir)
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		// Should still succeed as it uses the directory name
		assert.NoError(t, err)
	})
	
	t.Run("interactive_mode_flag", func(t *testing.T) {
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset and set interactive flag
		resetFlags()
		interactive = true
		
		// Note: We can't test the actual interactive mode here
		// as it requires user input. This just tests the flag is set.
		// The actual interactive wizard would need to be mocked.
		
		// For now, we'll just verify the flag is set correctly
		assert.True(t, interactive)
		
		// Reset for other tests
		interactive = false
	})
}

func TestConfigCreation(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	t.Run("validate_created_config", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset flags
		resetFlags()
		projectName = "validation-test"
		envName = "production"
		awsRegion = "us-west-2"
		awsProfile = "prod"
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Load and validate the created config
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		// Validate the config
		err = cfg.Validate()
		assert.NoError(t, err)
		
		// Check all expected values
		assert.Equal(t, "validation-test", cfg.Project)
		assert.Equal(t, "production", cfg.DefaultEnvironment)
		assert.Equal(t, "parameter_store", cfg.AWS.Service)
		assert.Equal(t, "us-west-2", cfg.AWS.Region)
		assert.Equal(t, "prod", cfg.AWS.Profile)
		
		// Check environment
		prodEnv, exists := cfg.Environments["production"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env.production"}, prodEnv.Files)
		assert.Equal(t, "/validation-test/production/", prodEnv.Path)
		assert.False(t, prodEnv.UseSecretsManager)
	})
	
	t.Run("env_file_permissions", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset flags
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Check .env.dev file permissions (only if created)
		if info, err := os.Stat(".env.dev"); err == nil {
			// Should be readable/writable by owner only (0600)
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
		}
	})
}

func TestOutputMessages(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	t.Run("success_output", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset flags
		resetFlags()
		projectName = "output-test"
		
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		w.Close()
		os.Stdout = oldStdout
		
		var output bytes.Buffer
		_, _ = output.ReadFrom(r)
		
		require.NoError(t, err)
		
		outputStr := output.String()
		assert.Contains(t, outputStr, "Successfully initialized envy project 'output-test'")
		assert.Contains(t, outputStr, "Created .envyrc configuration file")
		assert.Contains(t, outputStr, "Next steps:")
		assert.Contains(t, outputStr, "Edit .envyrc")
		assert.Contains(t, outputStr, ".env.dev")
		assert.Contains(t, outputStr, "envy push")
	})
}

// Helper function to reset flags to default values
func resetFlags() {
	projectName = ""
	envName = "dev"
	awsRegion = "us-east-1"
	awsProfile = "default"
	interactive = false
}

// Benchmark tests
func TestDetectExistingEnvFiles(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	t.Run("detect_multiple_env_files", func(t *testing.T) {
		tempDir := helper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Create multiple .env files
		testutil.WriteFile(t, tempDir, ".env", "DEFAULT=true")
		testutil.WriteFile(t, tempDir, ".env.local", "LOCAL=true")
		testutil.WriteFile(t, tempDir, ".env.prod", "PROD=true")
		testutil.WriteFile(t, tempDir, ".env.example", "EXAMPLE=true") // Should be ignored
		
		// Reset flags
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Load config and check environments
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		// Should have detected 3 environments (excluding .example)
		assert.Len(t, cfg.Environments, 3)
		
		// Check each environment
		defaultEnv, exists := cfg.Environments["default"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env"}, defaultEnv.Files)
		
		localEnv, exists := cfg.Environments["local"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env.local"}, localEnv.Files)
		
		prodEnv, exists := cfg.Environments["prod"]
		assert.True(t, exists)
		assert.Equal(t, []string{".env.prod"}, prodEnv.Files)
		
		// Should not create new .env files
		testutil.AssertFileNotExists(t, ".env.dev")
	})
	
	t.Run("no_existing_env_files", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Reset flags
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Should create default .env.dev file
		testutil.AssertFileExists(t, ".env.dev")
		
		// Load config
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		// Should have default dev environment
		assert.Len(t, cfg.Environments, 1)
		_, exists := cfg.Environments["dev"]
		assert.True(t, exists)
	})
	
	t.Run("detect_production_variants", func(t *testing.T) {
		// Create new helper for this subtest
		subHelper := testutil.NewTestHelper(t)
		defer subHelper.Cleanup()
		
		tempDir := subHelper.TempDir()
		testutil.ChangeDir(t, tempDir)
		
		// Create production variant files
		testutil.WriteFile(t, tempDir, ".env.production", "PRODUCTION=true")
		testutil.WriteFile(t, tempDir, ".env.development", "DEVELOPMENT=true")
		testutil.WriteFile(t, tempDir, ".env.staging", "STAGING=true")
		
		// Reset flags
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runInit(cmd, []string{})
		
		require.NoError(t, err)
		
		// Load config
		cfg, err := config.Load(".envyrc")
		require.NoError(t, err)
		
		// Check mapped environment names
		_, hasProd := cfg.Environments["prod"]
		assert.True(t, hasProd)
		
		_, hasDev := cfg.Environments["dev"]
		assert.True(t, hasDev)
		
		_, hasStaging := cfg.Environments["staging"]
		assert.True(t, hasStaging)
	})
}

func BenchmarkRunInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Create a temp directory for each iteration
		tempDir, err := os.MkdirTemp("", "envy-bench-*")
		if err != nil {
			b.Fatal(err)
		}
		
		// Change to temp directory
		os.Chdir(tempDir)
		
		// Reset flags
		resetFlags()
		
		// Run init
		cmd := &cobra.Command{}
		runInit(cmd, []string{})
		
		// Cleanup
		os.Chdir("..")
		os.RemoveAll(tempDir)
	}
}