//go:build integration
// +build integration

package integration

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

// TestIntegration_ConfigAndEnvFlow tests the full configuration and environment workflow
func TestIntegration_ConfigAndEnvFlow(t *testing.T) {
	// Setup test environment
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	// Create test configuration
	configContent := testutil.NewTestFixtures().ConfigYAML()
	configFile := testutil.WriteFile(t, tempDir, ".envyrc", configContent)

	// Create test environment files
	envContent := testutil.CreateTestEnvContent()
	testutil.WriteFile(t, tempDir, ".env.dev", envContent)
	testutil.WriteFile(t, tempDir, ".env.test", envContent)
	testutil.WriteFile(t, tempDir, ".env.prod", envContent)

	t.Run("config_loading_and_validation", func(t *testing.T) {
		// Load configuration
		cfg, err := config.Load(configFile)
		assert.NoError(t, err)
		require.NotNil(t, cfg)

		// Validate configuration
		err = cfg.Validate()
		assert.NoError(t, err)

		// Test environment access
		_, err = cfg.GetEnvironment("dev")
		assert.NoError(t, err)
		_, err = cfg.GetEnvironment("test")
		assert.NoError(t, err)
		_, err = cfg.GetEnvironment("prod")
		assert.NoError(t, err)
		_, err = cfg.GetEnvironment("nonexistent")
		assert.Error(t, err)
	})

	t.Run("env_file_parsing_and_manipulation", func(t *testing.T) {
		// Parse environment file
		envFile, err := env.ParseFile(filepath.Join(tempDir, ".env.dev"))
		assert.NoError(t, err)
		require.NotNil(t, envFile)

		// Verify parsed content
		expected := map[string]string{
			"APP_NAME":     "test-app",
			"DEBUG":        "true",
			"DATABASE_URL": "postgres://user:pass@localhost/db",
			"API_KEY":      "secret-key-123",
			"PORT":         "8080",
			"REDIS_URL":    "redis://localhost:6379",
		}

		actual := envFile.ToMap()
		testutil.AssertMapContains(t, actual, expected)

		// Test file manipulation
		envFile.Set("NEW_VAR", "new_value")
		envFile.Delete("REDIS_URL")

		// Write and verify
		outputFile := filepath.Join(tempDir, ".env.modified")
		err = envFile.WriteFile(outputFile)
		assert.NoError(t, err)

		// Parse modified file
		modifiedFile, err := env.ParseFile(outputFile)
		assert.NoError(t, err)

		value, exists := modifiedFile.Get("NEW_VAR")
		assert.True(t, exists)
		assert.Equal(t, "new_value", value)

		_, exists = modifiedFile.Get("REDIS_URL")
		assert.False(t, exists)
	})

	t.Run("file_merging_workflow", func(t *testing.T) {
		// Create base file
		baseFile := env.NewFile()
		baseFile.Set("BASE_VAR", "base_value")
		baseFile.Set("SHARED_VAR", "base_shared")

		// Create override file
		overrideFile := env.NewFile()
		overrideFile.Set("OVERRIDE_VAR", "override_value")
		overrideFile.Set("SHARED_VAR", "override_shared")

		// Merge files
		baseFile.Merge(overrideFile)

		// Verify merge results
		expected := map[string]string{
			"BASE_VAR":     "base_value",
			"OVERRIDE_VAR": "override_value",
			"SHARED_VAR":   "override_shared", // Should be overridden
		}

		actual := baseFile.ToMap()
		testutil.AssertMapEqual(t, expected, actual)
	})
}

// TestIntegration_PerformanceAndMemory tests performance and memory characteristics
func TestIntegration_PerformanceAndMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	t.Run("large_file_parsing", func(t *testing.T) {
		// Create large environment file
		fixtures := testutil.NewTestFixtures()
		largeContent := fixtures.EnvContentLarge(10000)

		tempFile := testutil.TempFile(t, largeContent)

		// Test parsing performance
		testutil.AssertPerformance(t, func() {
			file, err := env.ParseFile(tempFile)
			assert.NoError(t, err)
			assert.NotNil(t, file)
			testutil.AssertEnvFileSize(t, file, 10000)
		}, 2*time.Second, "parsing 10000 variables")
	})

	t.Run("memory_usage_with_large_files", func(t *testing.T) {
		testutil.AssertMemoryUsage(t, func() {
			// Create multiple large files
			for i := 0; i < 5; i++ {
				file := testutil.CreateLargeTestEnvFile(1000)
				vars, cleanup := file.ToMapWithPool()

				// Simulate processing
				for key, value := range vars {
					_ = key + "=" + value
				}

				cleanup()
			}
		}, 200) // 200MB limit
	})

	t.Run("concurrent_file_operations", func(t *testing.T) {
		envFile := testutil.CreateTestEnvFile()

		// Test concurrent read operations
		testutil.AssertConcurrentSafe(t, func() {
			envFile.Get("APP_NAME")
			envFile.ToMap()
			envFile.Keys()
			envFile.SortedKeys()
		}, 20, 100)
	})
}

// TestIntegration_ErrorHandlingAndRecovery tests error scenarios and recovery
func TestIntegration_ErrorHandlingAndRecovery(t *testing.T) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	t.Run("invalid_config_handling", func(t *testing.T) {
		// Create invalid configuration
		invalidConfig := testutil.NewTestFixtures().ConfigYAMLInvalid()
		invalidConfigFile := testutil.WriteFile(t, tempDir, ".envyrc.invalid", invalidConfig)

		// Should handle invalid config gracefully
		cfg, err := config.Load(invalidConfigFile)
		if err != nil {
			// Error should be descriptive
			assert.Contains(t, err.Error(), "failed to")
		} else {
			// If loaded, should fail validation
			err = cfg.Validate()
			assert.Error(t, err)
		}
	})

	t.Run("malformed_env_file_handling", func(t *testing.T) {
		// Create malformed env file
		malformedContent := testutil.NewTestFixtures().EnvContentMalformed()
		malformedFile := testutil.TempFile(t, malformedContent)

		// Should parse without error but skip invalid lines
		envFile, err := env.ParseFile(malformedFile)
		assert.NoError(t, err)
		require.NotNil(t, envFile)

		// Should have some valid variables (those that could be parsed)
		vars := envFile.ToMap()

		// Should not have variables with invalid names
		for key := range vars {
			assert.Regexp(t, `^[A-Za-z_][A-Za-z0-9_]*$`, key, "Variable name should be valid")
		}
	})

	t.Run("file_permission_handling", func(t *testing.T) {
		// Create test file
		testContent := "TEST_VAR=test_value"
		testFile := testutil.WriteFile(t, tempDir, "test.env", testContent)

		// Test reading with normal permissions
		envFile, err := env.ParseFile(testFile)
		assert.NoError(t, err)
		assert.NotNil(t, envFile)

		// Test writing with restricted permissions
		envFile.Set("NEW_VAR", "new_value")

		outputFile := filepath.Join(tempDir, "output.env")
		err = envFile.WriteFile(outputFile)
		assert.NoError(t, err)

		// Verify file permissions are secure (600)
		info, err := os.Stat(outputFile)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}

// TestIntegration_RealWorldScenarios tests realistic usage scenarios
func TestIntegration_RealWorldScenarios(t *testing.T) {
	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	t.Run("multi_environment_project", func(t *testing.T) {
		// Setup multi-environment project structure
		cfg := testutil.CreateTestConfig()

		// Create environment-specific files
		environments := map[string]string{
			"dev":  "DEBUG=true\nAPI_URL=http://localhost:3000\nDB_NAME=myapp_dev",
			"test": "DEBUG=true\nAPI_URL=http://test.example.com\nDB_NAME=myapp_test",
			"prod": "DEBUG=false\nAPI_URL=https://api.example.com\nDB_NAME=myapp_prod",
		}

		for envName, content := range environments {
			testutil.WriteFile(t, tempDir, ".env."+envName, content)
		}

		// Test each environment
		for envName := range environments {
			t.Run("env_"+envName, func(t *testing.T) {
				// Verify environment configuration
				envConfig, err := cfg.GetEnvironment(envName)
				assert.NoError(t, err)
				assert.NotNil(t, envConfig)

				// Parse environment file
				envFile, err := env.ParseFile(filepath.Join(tempDir, ".env."+envName))
				assert.NoError(t, err)
				require.NotNil(t, envFile)

				// Verify environment-specific values
				if envName == "prod" {
					debug, exists := envFile.Get("DEBUG")
					assert.True(t, exists)
					assert.Equal(t, "false", debug)
				} else {
					debug, exists := envFile.Get("DEBUG")
					assert.True(t, exists)
					assert.Equal(t, "true", debug)
				}

				// Verify API URLs are different
				apiURL, exists := envFile.Get("API_URL")
				assert.True(t, exists)
				if envName == "dev" {
					assert.Contains(t, apiURL, "localhost")
				} else {
					assert.Contains(t, apiURL, "example.com")
				}
			})
		}
	})

	t.Run("configuration_inheritance_and_override", func(t *testing.T) {
		// Create base configuration
		baseConfig := &config.Config{
			Project:            "inherited-project",
			DefaultEnvironment: "dev",
			AWS: config.AWSConfig{
				Service: "parameter_store",
				Region:  "us-east-1",
				Profile: "default",
			},
			Environments: map[string]config.Environment{
				"dev": {
					Files: []string{".env.dev"},
					Path:  "/inherited-project/dev/",
				},
				"prod": {
					Files:             []string{".env.prod"},
					Path:              "/inherited-project/prod/",
					UseSecretsManager: true,
				},
			},
		}

		// Test configuration inheritance
		assert.Equal(t, "parameter_store", baseConfig.GetAWSService("dev"))
		assert.Equal(t, "secrets_manager", baseConfig.GetAWSService("prod"))

		// Test path generation
		assert.Equal(t, "/inherited-project/dev/", baseConfig.GetParameterPath("dev"))
		assert.Equal(t, "/inherited-project/prod/", baseConfig.GetParameterPath("prod"))

		// Test default fallbacks
		assert.Equal(t, "parameter_store", baseConfig.GetAWSService("nonexistent"))
		assert.Equal(t, "/inherited-project/nonexistent/", baseConfig.GetParameterPath("nonexistent"))
	})

	t.Run("sensitive_data_handling", func(t *testing.T) {
		// Create environment file with sensitive data
		sensitiveContent := `# Application config
APP_NAME=sensitive-app
DEBUG=false

# Database credentials
DATABASE_URL=postgres://user:pass@localhost/db
DATABASE_PASSWORD=super-secret-password

# API keys and tokens
API_KEY=ak_12345_secret_key
JWT_SECRET=jwt_super_secret_signing_key
OAUTH_CLIENT_SECRET=oauth_client_secret_value

# Certificates and keys
PRIVATE_KEY=-----BEGIN PRIVATE KEY-----
SSL_CERTIFICATE=-----BEGIN CERTIFICATE-----

# Non-sensitive values
PORT=8080
HOST=localhost
LOG_LEVEL=info`

		envFile := testutil.TempFile(t, sensitiveContent)

		// Parse file
		parsedFile, err := env.ParseFile(envFile)
		assert.NoError(t, err)
		require.NotNil(t, parsedFile)

		// Identify sensitive vs non-sensitive variables
		sensitiveVars := testutil.NewTestFixtures().SensitiveVariableNames()

		vars := parsedFile.ToMap()

		// Test sensitive variable detection patterns
		for key := range vars {
			isSensitive := false
			for _, pattern := range sensitiveVars {
				if strings.Contains(strings.ToUpper(key), pattern) {
					isSensitive = true
					break
				}
			}

			if isSensitive {
				t.Logf("Detected sensitive variable: %s", key)
			}
		}

		// Verify file permissions when writing
		outputFile := filepath.Join(tempDir, "sensitive.env")
		err = parsedFile.WriteFile(outputFile)
		assert.NoError(t, err)

		info, err := os.Stat(outputFile)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "Sensitive files should have 600 permissions")
	})
}

// TestIntegration_BackwardCompatibility tests backward compatibility scenarios
func TestIntegration_BackwardCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compatibility tests in short mode")
	}

	tempDir := testutil.TempDir(t)
	testutil.ChangeDir(t, tempDir)

	t.Run("legacy_env_file_formats", func(t *testing.T) {
		// Test various .env file formats that should be supported
		legacyFormats := map[string]string{
			"basic": `
KEY1=value1
KEY2=value2
`,
			"with_comments": `
# This is a comment
KEY1=value1  # inline comment
KEY2=value2
`,
			"quoted_values": `
KEY1="quoted value"
KEY2='single quoted'
KEY3=""
`,
			"special_characters": `
URL=https://example.com/path?param=value
EMAIL=user@example.com
PATH=/usr/local/bin:/usr/bin
`,
			"multiline_and_escapes": `
MULTILINE="line1
line2
line3"
ESCAPED="value with \"quotes\""
`,
		}

		for formatName, content := range legacyFormats {
			t.Run(formatName, func(t *testing.T) {
				tempFile := testutil.TempFile(t, content)

				envFile, err := env.ParseFile(tempFile)
				assert.NoError(t, err, "Should parse %s format without error", formatName)
				require.NotNil(t, envFile)

				// Should have at least some variables (depending on format)
				vars := envFile.ToMap()
				assert.True(t, len(vars) > 0, "Should parse at least one variable from %s format", formatName)

				// Test round-trip (parse -> write -> parse)
				outputFile := filepath.Join(tempDir, formatName+".env")
				err = envFile.WriteFile(outputFile)
				assert.NoError(t, err)

				reparsedFile, err := env.ParseFile(outputFile)
				assert.NoError(t, err)

				// Should maintain variable integrity
				originalVars := envFile.ToMap()
				reparsedVars := reparsedFile.ToMap()
				testutil.AssertMapEqual(t, originalVars, reparsedVars)
			})
		}
	})
}

// BenchmarkIntegration_FullWorkflow benchmarks the complete workflow
func BenchmarkIntegration_FullWorkflow(b *testing.B) {
	tempDir := testutil.TempDir(&testing.T{})
	defer os.RemoveAll(tempDir)

	// Setup
	configContent := testutil.NewTestFixtures().ConfigYAML()
	configFile := testutil.WriteFile(&testing.T{}, tempDir, ".envyrc", configContent)

	envContent := testutil.NewTestFixtures().EnvContentLarge(100)
	envFile := testutil.WriteFile(&testing.T{}, tempDir, ".env.test", envContent)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Load config
		cfg, err := config.Load(configFile)
		if err != nil {
			b.Fatal(err)
		}

		// Parse env file
		parsedFile, err := env.ParseFile(envFile)
		if err != nil {
			b.Fatal(err)
		}

		// Process variables
		vars := parsedFile.ToMap()

		// Simulate AWS operations (configuration lookup)
		for envName := range cfg.Environments {
			cfg.GetAWSService(envName)
			cfg.GetParameterPath(envName)
		}

		// Simulate variable processing
		for key, value := range vars {
			_ = key + "=" + value
		}
	}
}
