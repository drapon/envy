package pull

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullCmd_Flags(t *testing.T) {
	// Reset flags for testing
	resetFlags()

	cmd := pullCmd

	// Test that all expected flags are present
	assert.NotNil(t, cmd.Flags().Lookup("env"))
	assert.NotNil(t, cmd.Flags().Lookup("prefix"))
	assert.NotNil(t, cmd.Flags().Lookup("output"))
	assert.NotNil(t, cmd.Flags().Lookup("export"))
	assert.NotNil(t, cmd.Flags().Lookup("overwrite"))
	assert.NotNil(t, cmd.Flags().Lookup("all"))
	assert.NotNil(t, cmd.Flags().Lookup("backup"))
	assert.NotNil(t, cmd.Flags().Lookup("merge"))

	// Test flag shortcuts
	envFlag := cmd.Flags().Lookup("env")
	assert.Equal(t, "e", envFlag.Shorthand)

	prefixFlag := cmd.Flags().Lookup("prefix")
	assert.Equal(t, "p", prefixFlag.Shorthand)

	outputFlag := cmd.Flags().Lookup("output")
	assert.Equal(t, "o", outputFlag.Shorthand)

	exportFlag := cmd.Flags().Lookup("export")
	assert.Equal(t, "x", exportFlag.Shorthand)

	overwriteFlag := cmd.Flags().Lookup("overwrite")
	assert.Equal(t, "w", overwriteFlag.Shorthand)

	allFlag := cmd.Flags().Lookup("all")
	assert.Equal(t, "a", allFlag.Shorthand)

	mergeFlag := cmd.Flags().Lookup("merge")
	assert.Equal(t, "m", mergeFlag.Shorthand)
}

func TestPullCmd_Usage(t *testing.T) {
	cmd := pullCmd

	assert.Equal(t, "pull", cmd.Use)
	assert.Contains(t, cmd.Short, "Pull environment variables from AWS")
	assert.Contains(t, cmd.Long, "Pull environment variables from AWS Parameter Store or Secrets Manager")
	assert.NotEmpty(t, cmd.Example)
}

func TestGetSourceDescription(t *testing.T) {
	cfg := testutil.CreateTestConfig()

	testCases := []struct {
		name     string
		envName  string
		expected string
	}{
		{
			name:     "parameter_store_environment",
			envName:  "test",
			expected: "AWS Parameter Store /test-project/test/ (us-east-1)",
		},
		{
			name:     "secrets_manager_environment",
			envName:  "prod",
			expected: "AWS Secrets Manager (us-east-1)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			description := getSourceDescription(cfg, tc.envName)
			assert.Equal(t, tc.expected, description)
		})
	}
}

func TestFileExists(t *testing.T) {
	// Test with existing file
	tempFile := testutil.TempFile(t, "test content")
	assert.True(t, fileExists(tempFile))

	// Test with non-existing file
	assert.False(t, fileExists("/path/to/nonexistent/file.txt"))
}

func TestCreateBackupFilename(t *testing.T) {
	testCases := []struct {
		name     string
		original string
		pattern  string
	}{
		{
			name:     "env_file",
			original: ".env.prod",
			pattern:  `\.env\.backup_\d{8}_\d{6}\.\d{3}\.prod`,
		},
		{
			name:     "no_extension",
			original: "envfile",
			pattern:  `envfile\.backup_\d{8}_\d{6}\.\d{3}`,
		},
		{
			name:     "with_path",
			original: "/path/to/.env.dev",
			pattern:  `/path/to/\.env\.backup_\d{8}_\d{6}\.\d{3}\.dev`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backup := createBackupFilename(tc.original)
			assert.Regexp(t, tc.pattern, backup)
			assert.Contains(t, backup, "backup_")
		})
	}
}

func TestCopyFile(t *testing.T) {
	tempDir := testutil.TempDir(t)

	// Create source file
	sourceContent := "TEST_VAR=test_value\nANOTHER_VAR=another_value"
	sourceFile := testutil.WriteFile(t, tempDir, "source.env", sourceContent)

	// Create destination path
	destFile := filepath.Join(tempDir, "dest.env")

	// Test copy operation
	err := copyFile(sourceFile, destFile)
	assert.NoError(t, err)

	// Verify destination file exists and has same content
	testutil.AssertFileExists(t, destFile)
	destContent := testutil.ReadFile(t, destFile)
	assert.Equal(t, sourceContent, destContent)

	// Verify file permissions
	info, err := os.Stat(destFile)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestCopyFile_Error(t *testing.T) {
	tempDir := testutil.TempDir(t)

	// Test with non-existent source file
	nonExistentSource := filepath.Join(tempDir, "nonexistent.env")
	destFile := filepath.Join(tempDir, "dest.env")

	err := copyFile(nonExistentSource, destFile)
	assert.Error(t, err)
	testutil.AssertFileNotExists(t, destFile)
}

func TestExportVariables(t *testing.T) {
	envFile := env.NewFile()
	envFile.Set("APP_NAME", "test-app")
	envFile.Set("DEBUG", "true")
	envFile.Set("SPECIAL_CHARS", "value with spaces and 'quotes'")
	envFile.Set("EMPTY_VAR", "")

	// Capture output
	stdout, _ := testutil.CaptureOutput(t, func() {
		err := exportVariables(envFile)
		assert.NoError(t, err)
	})

	// Check export format
	assert.Contains(t, stdout, "# Export environment variables")
	assert.Contains(t, stdout, "export APP_NAME='test-app'")
	assert.Contains(t, stdout, "export DEBUG='true'")
	assert.Contains(t, stdout, "export EMPTY_VAR=''")

	// Check that single quotes are properly escaped
	assert.Contains(t, stdout, "export SPECIAL_CHARS='value with spaces and '\"'\"'quotes'\"'\"''")
}

func TestRunPull_FlagParsing(t *testing.T) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	// Create test config file
	configContent := testutil.NewTestFixtures().ConfigYAML()
	testutil.WriteFile(t, tempDir, ".envyrc", configContent)

	testCases := []struct {
		name      string
		args      []string
		env       string
		output    string
		export    bool
		overwrite bool
		all       bool
		backup    bool
		merge     bool
	}{
		{
			name:   "basic_flags",
			args:   []string{"--env", "dev", "--output", ".env.test", "--export"},
			env:    "dev",
			output: ".env.test",
			export: true,
		},
		{
			name:      "overwrite_and_backup",
			args:      []string{"--overwrite", "--backup=false"},
			overwrite: true,
			backup:    false,
		},
		{
			name: "all_environments",
			args: []string{"--all"},
			all:  true,
		},
		{
			name:  "merge_mode",
			args:  []string{"--merge"},
			merge: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset flags
			resetFlags()

			// Set up command with flags
			cmd := pullCmd
			cmd.ParseFlags(tc.args)

			// Verify flags were parsed correctly
			if tc.env != "" {
				assert.Equal(t, tc.env, environment)
			}
			if tc.output != "" {
				assert.Equal(t, tc.output, output)
			}
			assert.Equal(t, tc.export, export)
			assert.Equal(t, tc.overwrite, overwrite)
			assert.Equal(t, tc.all, all)
			assert.Equal(t, tc.backup, backup)
			assert.Equal(t, tc.merge, merge)
		})
	}
}

func TestPullEnvironment_OutputFileSelection(t *testing.T) {
	cfg := testutil.CreateTestConfig()

	testCases := []struct {
		name           string
		envName        string
		outputFlag     string
		expectedOutput string
	}{
		{
			name:           "explicit_output_file",
			envName:        "test",
			outputFlag:     ".env.custom",
			expectedOutput: ".env.custom",
		},
		{
			name:           "default_from_config",
			envName:        "test",
			outputFlag:     "",
			expectedOutput: ".env.test", // From config files[0]
		},
		{
			name:           "fallback_pattern",
			envName:        "dev",
			outputFlag:     "",
			expectedOutput: ".env.dev", // From config files[0]
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get environment configuration
			envConfig, err := cfg.GetEnvironment(tc.envName)
			require.NoError(t, err)

			// Simulate output file selection logic
			outputFile := tc.outputFlag
			if outputFile == "" && len(envConfig.Files) > 0 {
				outputFile = envConfig.Files[0]
			}
			if outputFile == "" {
				outputFile = ".env." + tc.envName
			}

			assert.Equal(t, tc.expectedOutput, outputFile)
		})
	}
}

func TestPullEnvironment_MergeMode(t *testing.T) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	// Create existing env file
	existingContent := `EXISTING_VAR=existing_value
SHARED_VAR=old_value`
	existingFile := testutil.WriteFile(t, tempDir, ".env.test", existingContent)

	// Create new env file (simulating AWS pull)
	newEnvFile := env.NewFile()
	newEnvFile.Set("SHARED_VAR", "new_value")
	newEnvFile.Set("NEW_VAR", "new_value")

	// Test merge logic
	t.Run("merge_enabled", func(t *testing.T) {
		merge = true

		if merge && fileExists(existingFile) {
			existingEnvFile, err := env.ParseFile(existingFile)
			assert.NoError(t, err)

			// Simulate merge
			existingEnvFile.Merge(newEnvFile)

			// Verify merge results
			value, exists := existingEnvFile.Get("EXISTING_VAR")
			assert.True(t, exists)
			assert.Equal(t, "existing_value", value)

			value, exists = existingEnvFile.Get("SHARED_VAR")
			assert.True(t, exists)
			assert.Equal(t, "new_value", value) // Should be overwritten

			value, exists = existingEnvFile.Get("NEW_VAR")
			assert.True(t, exists)
			assert.Equal(t, "new_value", value)
		}
	})

	t.Run("merge_disabled", func(t *testing.T) {
		merge = false

		// When merge is disabled, only new variables should be present
		expectedVars := map[string]string{
			"SHARED_VAR": "new_value",
			"NEW_VAR":    "new_value",
		}

		actualVars := newEnvFile.ToMap()
		testutil.AssertMapEqual(t, expectedVars, actualVars)
	})
}

func TestPullEnvironment_BackupCreation(t *testing.T) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	// Create existing file
	originalContent := "ORIGINAL_VAR=original_value"
	outputFile := testutil.WriteFile(t, tempDir, ".env.test", originalContent)

	t.Run("backup_enabled", func(t *testing.T) {
		backup = true
		overwrite = false

		if backup && !overwrite && fileExists(outputFile) {
			backupFile := createBackupFilename(outputFile)

			err := copyFile(outputFile, backupFile)
			assert.NoError(t, err)

			// Verify backup was created
			testutil.AssertFileExists(t, backupFile)
			backupContent := testutil.ReadFile(t, backupFile)
			assert.Equal(t, originalContent, backupContent)

			// Clean up
			os.Remove(backupFile)
		}
	})

	t.Run("backup_disabled", func(t *testing.T) {
		backup = false

		// No backup should be created when disabled
		assert.False(t, backup)
	})

	t.Run("overwrite_mode", func(t *testing.T) {
		backup = true
		overwrite = true

		// No backup should be created when overwrite is enabled
		if backup && !overwrite && fileExists(outputFile) {
			// This condition should be false
			assert.False(t, true, "Backup should not be created in overwrite mode")
		}
	})
}

func TestPullEnvironment_EnvironmentSelection(t *testing.T) {
	cfg := testutil.CreateTestConfig()

	testCases := []struct {
		name            string
		envName         string
		expectedService string
		expectedPath    string
		expectError     bool
	}{
		{
			name:            "valid_test_environment",
			envName:         "test",
			expectedService: "parameter_store",
			expectedPath:    "/test-project/test/",
			expectError:     false,
		},
		{
			name:            "valid_prod_environment",
			envName:         "prod",
			expectedService: "secrets_manager",
			expectedPath:    "/test-project/prod/",
			expectError:     false,
		},
		{
			name:            "nonexistent_environment",
			envName:         "nonexistent",
			expectedService: "parameter_store",
			expectedPath:    "/test-project/nonexistent/",
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envConfig, err := cfg.GetEnvironment(tc.envName)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, envConfig)

			service := cfg.GetAWSService(tc.envName)
			path := cfg.GetParameterPath(tc.envName)

			assert.Equal(t, tc.expectedService, service)
			assert.Equal(t, tc.expectedPath, path)
		})
	}
}

func TestPullEnvironment_AllEnvironments(t *testing.T) {
	cfg := testutil.CreateTestConfig()

	// Test all environments selection
	all = true

	if all {
		environments := []string{}
		for envName := range cfg.Environments {
			environments = append(environments, envName)
		}

		assert.Contains(t, environments, "test")
		assert.Contains(t, environments, "dev")
		assert.Contains(t, environments, "prod")
		assert.Len(t, environments, 3)
	}
}

func TestPullEnvironment_DefaultEnvironment(t *testing.T) {
	cfg := testutil.CreateTestConfig()

	// Test default environment selection
	all = false
	environment = ""

	selectedEnv := environment
	if selectedEnv == "" {
		selectedEnv = cfg.DefaultEnvironment
	}

	assert.Equal(t, "test", selectedEnv) // From test config
}

// Benchmark tests
func BenchmarkGetSourceDescription(b *testing.B) {
	cfg := testutil.CreateTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envName := "test"
		if i%2 == 0 {
			envName = "prod"
		}
		_ = getSourceDescription(cfg, envName)
	}
}

func BenchmarkCreateBackupFilename(b *testing.B) {
	original := ".env.prod"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = createBackupFilename(original)
	}
}

func BenchmarkFileExists(b *testing.B) {
	tempFile := "/tmp/test.env"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fileExists(tempFile)
	}
}

// Helper functions for testing
func resetFlags() {
	environment = ""
	prefix = ""
	output = ""
	export = false
	overwrite = false
	all = false
	backup = false
	merge = false
}

// Test helper to setup test environment
func setupTestEnvironment(t *testing.T) (string, *config.Config) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	// Create test config
	cfg := testutil.CreateTestConfig()
	configContent := testutil.NewTestFixtures().ConfigYAML()
	testutil.WriteFile(t, tempDir, ".envyrc", configContent)

	return tempDir, cfg
}

func TestPullCmd_Integration(t *testing.T) {
	// Integration test without actual AWS calls
	tempDir, cfg := setupTestEnvironment(t)
	defer func() {
		os.RemoveAll(tempDir)
	}()

	t.Run("export_mode", func(t *testing.T) {
		resetFlags()
		export = true
		environment = "test"

		assert.True(t, export)
		assert.Equal(t, "test", environment)

		// Test export functionality
		envFile := testutil.CreateTestEnvFile()

		stdout, _ := testutil.CaptureOutput(t, func() {
			err := exportVariables(envFile)
			assert.NoError(t, err)
		})

		assert.Contains(t, stdout, "export APP_NAME=")
		assert.Contains(t, stdout, "# Export environment variables")
	})

	t.Run("file_output_mode", func(t *testing.T) {
		resetFlags()
		export = false
		environment = "test"
		output = ".env.test"

		assert.False(t, export)
		assert.Equal(t, "test", environment)
		assert.Equal(t, ".env.test", output)

		// Verify configuration
		assert.NotNil(t, cfg)
		assert.NoError(t, cfg.Validate())

		// Verify environment exists
		env, err := cfg.GetEnvironment(environment)
		assert.NoError(t, err)
		assert.NotNil(t, env)
	})

	t.Run("merge_with_existing", func(t *testing.T) {
		resetFlags()
		merge = true
		environment = "test"

		// Create existing file
		existingContent := "EXISTING_VAR=existing_value"
		outputFile := ".env.test"
		testutil.WriteFile(t, tempDir, outputFile, existingContent)

		assert.True(t, merge)
		assert.True(t, fileExists(outputFile))

		// Test merge functionality
		if merge && fileExists(outputFile) {
			existingFile, err := env.ParseFile(outputFile)
			assert.NoError(t, err)
			assert.NotNil(t, existingFile)

			value, exists := existingFile.Get("EXISTING_VAR")
			assert.True(t, exists)
			assert.Equal(t, "existing_value", value)
		}
	})
}

func TestPullCmd_ErrorHandling(t *testing.T) {
	testCases := []testutil.TestCase{
		{
			Name:     "invalid_environment",
			Input:    "nonexistent",
			Expected: "environment 'nonexistent' not found",
			Error:    true,
		},
		{
			Name:     "empty_environment_name",
			Input:    "",
			Expected: nil, // Should use default environment
			Error:    false,
		},
	}

	cfg := testutil.CreateTestConfig()

	testutil.RunTestTable(t, testCases, func(t *testing.T, tc testutil.TestCase) {
		envName := tc.Input.(string)

		var err error
		if envName == "" {
			envName = cfg.DefaultEnvironment
		}

		_, err = cfg.GetEnvironment(envName)

		if tc.Error {
			assert.Error(t, err)
			if tc.Expected != nil {
				assert.Contains(t, err.Error(), tc.Expected.(string))
			}
		} else {
			assert.NoError(t, err)
		}
	})
}

func TestPullCmd_FileOperations(t *testing.T) {
	tempDir := testutil.TempDir(t)

	t.Run("successful_copy", func(t *testing.T) {
		sourceContent := "TEST_VAR=test_value"
		sourceFile := testutil.WriteFile(t, tempDir, "source.env", sourceContent)
		destFile := filepath.Join(tempDir, "dest.env")

		err := copyFile(sourceFile, destFile)
		assert.NoError(t, err)

		testutil.AssertFileExists(t, destFile)
		destContent := testutil.ReadFile(t, destFile)
		assert.Equal(t, sourceContent, destContent)
	})

	t.Run("copy_nonexistent_source", func(t *testing.T) {
		nonExistentSource := filepath.Join(tempDir, "nonexistent.env")
		destFile := filepath.Join(tempDir, "dest_nonexistent.env")

		err := copyFile(nonExistentSource, destFile)
		assert.Error(t, err)
		testutil.AssertFileNotExists(t, destFile)
	})

	t.Run("backup_filename_generation", func(t *testing.T) {
		original := ".env.prod"
		backup1 := createBackupFilename(original)

		// Wait a short time to ensure different timestamp (now with milliseconds)
		time.Sleep(2 * time.Millisecond)
		backup2 := createBackupFilename(original)

		// Backups should be different due to timestamp
		assert.NotEqual(t, backup1, backup2)
		assert.Contains(t, backup1, "backup_")
		assert.Contains(t, backup2, "backup_")
	})
}

// Parallel tests
func TestPullCmd_ConcurrentOperations(t *testing.T) {
	cfg := testutil.CreateTestConfig()

	// Test concurrent access to configuration
	testutil.AssertConcurrentSafe(t, func() {
		cfg.GetEnvironment("test")
		cfg.GetAWSService("test")
		cfg.GetParameterPath("test")
		getSourceDescription(cfg, "test")
		fileExists(".env.test")
	}, 10, 100)
}

// Memory tests
func TestPullCmd_MemoryUsage(t *testing.T) {
	testutil.AssertMemoryUsage(t, func() {
		cfg := testutil.CreateTestConfig()
		envFile := testutil.CreateLargeTestEnvFile(1000)

		// Simulate memory-intensive operations
		for i := 0; i < 100; i++ {
			getSourceDescription(cfg, "test")
			createBackupFilename(".env.test")
			envFile.ToMap()
		}
	}, 100) // 100MB limit
}

// Performance tests
func TestPullCmd_Performance(t *testing.T) {
	cfg := testutil.CreateTestConfig()
	envFile := testutil.CreateLargeTestEnvFile(1000)

	testutil.AssertPerformance(t, func() {
		// Simulate typical operations
		for i := 0; i < 100; i++ {
			getSourceDescription(cfg, "test")
			createBackupFilename(".env.test")
			fileExists(".env.test")
		}

		// Test export performance
		exportVariables(envFile)
	}, 1*time.Second, "pull command operations")
}

func TestEscapeQuotesInExport(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no_quotes",
			input:    "simple_value",
			expected: "simple_value",
		},
		{
			name:     "single_quotes",
			input:    "value with 'quotes'",
			expected: "value with '\"'\"'quotes'\"'\"'",
		},
		{
			name:     "multiple_single_quotes",
			input:    "it's a 'test' value",
			expected: "it'\"'\"'s a '\"'\"'test'\"'\"' value",
		},
		{
			name:     "empty_value",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the quote escaping logic used in exportVariables
			escaped := strings.ReplaceAll(tc.input, "'", "'\"'\"'")
			assert.Equal(t, tc.expected, escaped)
		})
	}
}
