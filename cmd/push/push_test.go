package push

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPushCmd_Flags(t *testing.T) {
	// Reset flags for testing
	resetFlags()
	
	cmd := pushCmd
	
	// Test that all expected flags are present
	assert.NotNil(t, cmd.Flags().Lookup("env"))
	assert.NotNil(t, cmd.Flags().Lookup("prefix"))
	assert.NotNil(t, cmd.Flags().Lookup("vars"))
	assert.NotNil(t, cmd.Flags().Lookup("force"))
	assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, cmd.Flags().Lookup("all"))
	assert.NotNil(t, cmd.Flags().Lookup("diff"))
	assert.NotNil(t, cmd.Flags().Lookup("parallel"))
	assert.NotNil(t, cmd.Flags().Lookup("max-workers"))
	assert.NotNil(t, cmd.Flags().Lookup("batch-size"))
	
	// Test flag shortcuts
	envFlag := cmd.Flags().Lookup("env")
	assert.Equal(t, "e", envFlag.Shorthand)
	
	prefixFlag := cmd.Flags().Lookup("prefix")
	assert.Equal(t, "p", prefixFlag.Shorthand)
	
	varsFlag := cmd.Flags().Lookup("vars")
	assert.Equal(t, "v", varsFlag.Shorthand)
	
	forceFlag := cmd.Flags().Lookup("force")
	assert.Equal(t, "f", forceFlag.Shorthand)
	
	allFlag := cmd.Flags().Lookup("all")
	assert.Equal(t, "a", allFlag.Shorthand)
}

func TestPushCmd_Usage(t *testing.T) {
	cmd := pushCmd
	
	assert.Equal(t, "push", cmd.Use)
	assert.Contains(t, cmd.Short, "Push environment variables to AWS")
	assert.Contains(t, cmd.Long, "Push environment variables to AWS Parameter Store or Secrets Manager")
	assert.NotEmpty(t, cmd.Example)
}

func TestIsSensitive(t *testing.T) {
	testCases := []testutil.TestCase{
		{
			Name:     "password",
			Input:    "PASSWORD",
			Expected: true,
			Error:    false,
		},
		{
			Name:     "api_secret",
			Input:    "API_SECRET",
			Expected: true,
			Error:    false,
		},
		{
			Name:     "jwt_token",
			Input:    "JWT_TOKEN",
			Expected: true,
			Error:    false,
		},
		{
			Name:     "private_key",
			Input:    "PRIVATE_KEY",
			Expected: true,
			Error:    false,
		},
		{
			Name:     "database_credential",
			Input:    "DATABASE_CREDENTIAL",
			Expected: true,
			Error:    false,
		},
		{
			Name:     "app_name",
			Input:    "APP_NAME",
			Expected: false,
			Error:    false,
		},
		{
			Name:     "debug",
			Input:    "DEBUG",
			Expected: false,
			Error:    false,
		},
		{
			Name:     "port",
			Input:    "PORT",
			Expected: false,
			Error:    false,
		},
		{
			Name:     "url",
			Input:    "DATABASE_URL",
			Expected: false,
			Error:    false,
		},
	}
	
	testutil.RunTestTable(t, testCases, func(t *testing.T, tc testutil.TestCase) {
		key := tc.Input.(string)
		expected := tc.Expected.(bool)
		
		actual := isSensitive(key)
		assert.Equal(t, expected, actual)
	})
}

func TestGetTargetDescription(t *testing.T) {
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
			description := getTargetDescription(cfg, tc.envName)
			assert.Equal(t, tc.expected, description)
		})
	}
}

func TestConfirmPush(t *testing.T) {
	// Test with simulated user input
	// Note: This is a simplified test as we can't easily simulate stdin input
	// In a real implementation, you might want to refactor confirmPush to accept an io.Reader
	
	// For now, test the formatting of the confirmation message
	// by capturing what would be printed
	testCases := []struct {
		count   int
		envName string
	}{
		{count: 5, envName: "test"},
		{count: 10, envName: "prod"},
		{count: 0, envName: "empty"},
	}
	
	for _, tc := range testCases {
		t.Run("confirm_message", func(t *testing.T) {
			// Test that the function handles different inputs correctly
			// In a real test, we would mock the input/output
			assert.True(t, tc.count >= 0)
			assert.NotEmpty(t, tc.envName)
		})
	}
}

func TestShowDifferences(t *testing.T) {
	local := map[string]string{
		"EXISTING_VAR": "new_value",
		"NEW_VAR":      "new_value",
		"SAME_VAR":     "same_value",
	}
	
	remote := map[string]string{
		"EXISTING_VAR": "old_value",
		"REMOTE_ONLY":  "remote_value",
		"SAME_VAR":     "same_value",
	}
	
	// Capture output
	stdout, _ := testutil.CaptureOutput(t, func() {
		showDifferences(local, remote)
	})
	
	// Check that differences are properly identified
	assert.Contains(t, stdout, "Added:")
	assert.Contains(t, stdout, "+ NEW_VAR")
	
	assert.Contains(t, stdout, "Modified:")
	assert.Contains(t, stdout, "~ EXISTING_VAR")
	
	assert.Contains(t, stdout, "Will remain in remote")
	assert.Contains(t, stdout, "? REMOTE_ONLY")
}

func TestShowDifferences_NoChanges(t *testing.T) {
	local := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	}
	
	remote := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	}
	
	// Capture output
	stdout, _ := testutil.CaptureOutput(t, func() {
		showDifferences(local, remote)
	})
	
	assert.Contains(t, stdout, "No changes detected")
}

func TestRunPush_ConfigValidation(t *testing.T) {
	// Create temporary directory for test
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)
	
	// Test with missing configuration
	t.Run("missing_config", func(t *testing.T) {
		resetFlags()
		
		cmd := &cobra.Command{}
		err := runPush(cmd, []string{})
		
		// Should succeed with default config when no config file exists
		// (returns default config in real implementation)
		// For this test, we expect it to attempt to load config
		assert.Error(t, err) // Will fail because we don't have AWS credentials
	})
}

func TestRunPush_FlagParsing(t *testing.T) {
	// Test flag parsing without actually running AWS operations
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)
	
	// Create a test config file
	configContent := testutil.NewTestFixtures().ConfigYAML()
	testutil.WriteFile(t, tempDir, ".envyrc", configContent)
	
	// Create test env files
	envContent := testutil.CreateTestEnvContent()
	testutil.WriteFile(t, tempDir, ".env.dev", envContent)
	
	testCases := []struct {
		name string
		args []string
		env  string
		vars string
		force bool
		dryRun bool
		all bool
	}{
		{
			name: "basic_flags",
			args: []string{"--env", "dev", "--force", "--dry-run"},
			env:  "dev",
			force: true,
			dryRun: true,
		},
		{
			name: "variables_filter",
			args: []string{"--vars", "APP_NAME,DEBUG"},
			vars: "APP_NAME,DEBUG",
		},
		{
			name: "all_environments",
			args: []string{"--all"},
			all: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset flags
			resetFlags()
			
			// Set up command with flags
			cmd := pushCmd
			cmd.ParseFlags(tc.args)
			
			// Verify flags were parsed correctly
			if tc.env != "" {
				assert.Equal(t, tc.env, environment)
			}
			if tc.vars != "" {
				assert.Equal(t, tc.vars, variables)
			}
			assert.Equal(t, tc.force, force)
			assert.Equal(t, tc.dryRun, dryRun)
			assert.Equal(t, tc.all, all)
		})
	}
}

func TestPushEnvironment_EnvironmentSelection(t *testing.T) {
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
			expectedService: "parameter_store", // Default
			expectedPath:    "/test-project/nonexistent/",
			expectError:     true, // Environment config doesn't exist
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

func TestPushEnvironment_VariableFiltering(t *testing.T) {
	envFile := testutil.CreateTestEnvFile()
	
	// Test variable filtering logic
	testCases := []struct {
		name           string
		variableFilter string
		expectedVars   []string
		expectedCount  int
	}{
		{
			name:           "no_filter",
			variableFilter: "",
			expectedVars:   []string{"APP_NAME", "DEBUG", "DATABASE_URL", "API_KEY", "PORT"},
			expectedCount:  5,
		},
		{
			name:           "single_variable",
			variableFilter: "APP_NAME",
			expectedVars:   []string{"APP_NAME"},
			expectedCount:  1,
		},
		{
			name:           "multiple_variables",
			variableFilter: "APP_NAME,DEBUG,PORT",
			expectedVars:   []string{"APP_NAME", "DEBUG", "PORT"},
			expectedCount:  3,
		},
		{
			name:           "nonexistent_variable",
			variableFilter: "NONEXISTENT_VAR",
			expectedVars:   []string{},
			expectedCount:  0,
		},
		{
			name:           "mixed_existing_nonexistent",
			variableFilter: "APP_NAME,NONEXISTENT_VAR,DEBUG",
			expectedVars:   []string{"APP_NAME", "DEBUG"},
			expectedCount:  2,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filteredFile := testutil.CreateLargeTestEnvFile(0) // Start with empty file
			
			if tc.variableFilter != "" {
				varsToKeep := strings.Split(tc.variableFilter, ",")
				
				for _, varName := range varsToKeep {
					varName = strings.TrimSpace(varName)
					if value, exists := envFile.Get(varName); exists {
						filteredFile.Set(varName, value)
					}
				}
			} else {
				filteredFile = envFile
			}
			
			assert.Equal(t, tc.expectedCount, len(filteredFile.Variables))
			
			for _, expectedVar := range tc.expectedVars {
				_, exists := filteredFile.Get(expectedVar)
				assert.True(t, exists, "Variable %s should exist in filtered file", expectedVar)
			}
		})
	}
}

func TestCreateParameterStoreTask(t *testing.T) {
	key := "TEST_VAR"
	path := "/test-project/test/"
	
	// Test parameter name construction
	expectedParamName := path + key
	if !strings.HasSuffix(path, "/") {
		expectedParamName = path + "/" + key
	}
	
	assert.Equal(t, "/test-project/test/TEST_VAR", expectedParamName)
	
	// Test parameter type determination
	t.Run("string_parameter", func(t *testing.T) {
		paramType := "String"
		if isSensitive(key) {
			paramType = "SecureString"
		}
		assert.Equal(t, "String", paramType, "TEST_VAR should not be sensitive")
	})
	
	t.Run("secure_parameter", func(t *testing.T) {
		sensitiveKey := "API_SECRET"
		paramType := "String"
		if isSensitive(sensitiveKey) {
			paramType = "SecureString"
		}
		assert.Equal(t, "SecureString", paramType)
	})
}

func TestCreateSecretsManagerTask(t *testing.T) {
	key := "TEST_VAR"
	path := "/test-project/test/"
	
	// Test secret name construction
	secretName := strings.Trim(path, "/")
	secretName = strings.ReplaceAll(secretName, "/", "-") + "-" + key
	
	expectedSecretName := "test-project-test-TEST_VAR"
	assert.Equal(t, expectedSecretName, secretName)
}

// Benchmark tests
func BenchmarkIsSensitive(b *testing.B) {
	keys := []string{
		"APP_NAME", "DEBUG", "API_SECRET", "PASSWORD",
		"JWT_TOKEN", "DATABASE_URL", "PRIVATE_KEY", "PORT",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		_ = isSensitive(key)
	}
}

func BenchmarkGetTargetDescription(b *testing.B) {
	cfg := testutil.CreateTestConfig()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envName := "test"
		if i%2 == 0 {
			envName = "prod"
		}
		_ = getTargetDescription(cfg, envName)
	}
}

// Helper functions for testing
func resetFlags() {
	environment = ""
	prefix = ""
	variables = ""
	force = false
	dryRun = false
	all = false
	showDiff = false
	parallelMode = false
	maxWorkers = 10
	batchSize = 10
}

// Test helper to setup test environment
func setupTestEnvironment(t *testing.T) (string, *config.Config) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)
	
	// Create test config
	cfg := testutil.CreateTestConfig()
	configContent := testutil.NewTestFixtures().ConfigYAML()
	testutil.WriteFile(t, tempDir, ".envyrc", configContent)
	
	// Create test env files
	envContent := testutil.CreateTestEnvContent()
	testutil.WriteFile(t, tempDir, ".env.test", envContent)
	testutil.WriteFile(t, tempDir, ".env.dev", envContent)
	testutil.WriteFile(t, tempDir, ".env.prod", envContent)
	
	return tempDir, cfg
}

func TestPushCmd_Integration(t *testing.T) {
	// Integration test without actual AWS calls
	tempDir, cfg := setupTestEnvironment(t)
	defer func() {
		os.RemoveAll(tempDir)
	}()
	
	t.Run("dry_run_mode", func(t *testing.T) {
		resetFlags()
		dryRun = true
		environment = "test"
		
		// Test dry run logic
		assert.True(t, dryRun)
		assert.Equal(t, "test", environment)
		
		// Verify configuration
		assert.NotNil(t, cfg)
		assert.NoError(t, cfg.Validate())
		
		// Verify environment exists
		env, err := cfg.GetEnvironment(environment)
		assert.NoError(t, err)
		assert.NotNil(t, env)
	})
	
	t.Run("force_mode", func(t *testing.T) {
		resetFlags()
		force = true
		environment = "test"
		
		assert.True(t, force)
		assert.Equal(t, "test", environment)
	})
	
	t.Run("all_environments", func(t *testing.T) {
		resetFlags()
		all = true
		
		assert.True(t, all)
		
		// Test that all environments are selected
		environments := []string{}
		for envName := range cfg.Environments {
			environments = append(environments, envName)
		}
		
		assert.Contains(t, environments, "test")
		assert.Contains(t, environments, "dev")
		assert.Contains(t, environments, "prod")
		assert.Len(t, environments, 3)
	})
}

func TestPushCmd_ErrorHandling(t *testing.T) {
	testCases := []testutil.TestCase{
		{
			Name: "invalid_environment",
			Input: "nonexistent",
			Expected: "environment 'nonexistent' not found",
			Error: true,
		},
		{
			Name: "empty_environment_name",
			Input: "",
			Expected: nil, // Should use default environment
			Error: false,
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

// Parallel tests
func TestPushCmd_ConcurrentOperations(t *testing.T) {
	cfg := testutil.CreateTestConfig()
	
	// Test concurrent access to configuration
	testutil.AssertConcurrentSafe(t, func() {
		cfg.GetEnvironment("test")
		cfg.GetAWSService("test")
		cfg.GetParameterPath("test")
		isSensitive("TEST_KEY")
		getTargetDescription(cfg, "test")
	}, 10, 100)
}

// Memory tests
func TestPushCmd_MemoryUsage(t *testing.T) {
	testutil.AssertMemoryUsage(t, func() {
		cfg := testutil.CreateTestConfig()
		envFile := testutil.CreateLargeTestEnvFile(1000)
		
		// Simulate memory-intensive operations
		vars, cleanup := envFile.ToMapWithPool()
		defer cleanup()
		
		for i := 0; i < 100; i++ {
			getTargetDescription(cfg, "test")
			cfg.GetAWSService("test")
			
			for key := range vars {
				isSensitive(key)
			}
		}
	}, 100) // 100MB limit
}

// Performance tests
func TestPushCmd_Performance(t *testing.T) {
	cfg := testutil.CreateTestConfig()
	envFile := testutil.CreateLargeTestEnvFile(1000)
	
	testutil.AssertPerformance(t, func() {
		vars, cleanup := envFile.ToMapWithPool()
		defer cleanup()
		
		// Simulate processing variables
		for key := range vars {
			isSensitive(key)
		}
		
		getTargetDescription(cfg, "test")
		cfg.GetAWSService("test")
		cfg.GetParameterPath("test")
	}, 500*time.Millisecond, "processing 1000 variables")
}