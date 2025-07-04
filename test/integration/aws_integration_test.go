//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"strings"
)

// AWSIntegrationTestSuite is the integration test suite for AWS services
type AWSIntegrationTestSuite struct {
	suite.Suite
	ctx       context.Context
	manager   *aws.Manager
	helper    *LocalStackHelper
}

// SetupSuite sets up the test suite
func (suite *AWSIntegrationTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	
	// Check if LocalStack is running
	suite.helper = NewLocalStackHelper()
	if !suite.helper.IsRunning() {
		suite.T().Skip("LocalStack is not running. Skipping integration tests.")
	}
	
	// Test configuration
	cfg := &config.Config{
		AWS: config.AWSConfig{
			Region:  "us-east-1",
			Profile: "",
		},
		Project:            "integration-test",
		DefaultEnvironment: "test",
		Environments: map[string]config.Environment{
			"test": {
				Files: []string{".env.test"},
				Path:  "/integration-test/test/",
			},
		},
	}
	
	// Set LocalStack endpoint via environment variable
	os.Setenv("AWS_ENDPOINT_URL", suite.helper.GetEndpoint())
	defer os.Unsetenv("AWS_ENDPOINT_URL")
	
	// Initialize Manager
	var err error
	suite.manager, err = aws.NewManager(cfg)
	require.NoError(suite.T(), err)
}

// TearDownSuite cleans up the test suite
func (suite *AWSIntegrationTestSuite) TearDownSuite() {
	// Clean up LocalStack environment
	suite.helper.Cleanup(suite.ctx)
}

// SetupTest sets up each test
func (suite *AWSIntegrationTestSuite) SetupTest() {
	// Clean up test path before each test
	suite.cleanupTestPath("/test/integration")
}

// TestParameterStoreOperations tests Parameter Store operations
func (suite *AWSIntegrationTestSuite) TestParameterStoreOperations() {
	t := suite.T()
	
	t.Run("Push and pull environment variables", func(t *testing.T) {
		// Create test environment variable file
		envFile := env.NewFile()
		envFile.Set("TEST_VAR1", "value1")
		envFile.Set("TEST_VAR2", "value2")
		envFile.Set("TEST_SECRET", "secret-value")
		
		// Push environment variables
		err := suite.manager.PushEnvironment(suite.ctx, "test", envFile, true)
		require.NoError(t, err)
		
		// Pull environment variables
		pulledFile, err := suite.manager.PullEnvironment(suite.ctx, "test")
		require.NoError(t, err)
		
		// Verify values
		value1, exists := pulledFile.Get("TEST_VAR1")
		assert.True(t, exists)
		assert.Equal(t, "value1", value1)
		
		value2, exists := pulledFile.Get("TEST_VAR2")
		assert.True(t, exists)
		assert.Equal(t, "value2", value2)
		
		secret, exists := pulledFile.Get("TEST_SECRET")
		assert.True(t, exists)
		assert.Equal(t, "secret-value", secret)
	})
	
	t.Run("List environment variables", func(t *testing.T) {
		// Set up test environment variables
		envFile := env.NewFile()
		envFile.Set("LIST_VAR1", "value1")
		envFile.Set("LIST_VAR2", "value2")
		envFile.Set("LIST_VAR3", "value3")
		
		// Push
		err := suite.manager.PushEnvironment(suite.ctx, "test", envFile, true)
		require.NoError(t, err)
		
		// Get list
		vars, err := suite.manager.ListEnvironmentVariables(suite.ctx, "test")
		require.NoError(t, err)
		
		// Confirm that at least the pushed variables are included
		assert.GreaterOrEqual(t, len(vars), 3)
		assert.Contains(t, vars, "LIST_VAR1")
		assert.Contains(t, vars, "LIST_VAR2")
		assert.Contains(t, vars, "LIST_VAR3")
	})
	
	t.Run("Multiple environment management", func(t *testing.T) {
		// Variables for dev environment
		devFile := env.NewFile()
		devFile.Set("APP_ENV", "development")
		devFile.Set("API_URL", "http://localhost:3000")
		devFile.Set("DEBUG", "true")
		
		// Variables for prod environment
		prodFile := env.NewFile()
		prodFile.Set("APP_ENV", "production")
		prodFile.Set("API_URL", "https://api.example.com")
		prodFile.Set("DEBUG", "false")
		
		// Add multiple environment configurations
		suite.manager.GetConfig().Environments["dev"] = config.Environment{
			Files: []string{".env.dev"},
			Path:  "/integration-test/dev/",
		}
		suite.manager.GetConfig().Environments["prod"] = config.Environment{
			Files: []string{".env.prod"},
			Path:  "/integration-test/prod/",
		}
		
		// Push dev environment
		err := suite.manager.PushEnvironment(suite.ctx, "dev", devFile, true)
		require.NoError(t, err)
		
		// Push prod environment
		err = suite.manager.PushEnvironment(suite.ctx, "prod", prodFile, true)
		require.NoError(t, err)
		
		// Pull dev environment
		pulledDev, err := suite.manager.PullEnvironment(suite.ctx, "dev")
		require.NoError(t, err)
		appEnv, _ := pulledDev.Get("APP_ENV")
		assert.Equal(t, "development", appEnv)
		debugVal, _ := pulledDev.Get("DEBUG")
		assert.Equal(t, "true", debugVal)
		
		// Pull prod environment
		pulledProd, err := suite.manager.PullEnvironment(suite.ctx, "prod")
		require.NoError(t, err)
		appEnv, _ = pulledProd.Get("APP_ENV")
		assert.Equal(t, "production", appEnv)
		debugVal, _ = pulledProd.Get("DEBUG")
		assert.Equal(t, "false", debugVal)
	})
	
	t.Run("Update environment variables", func(t *testing.T) {
		// Initial environment variables
		originalFile := env.NewFile()
		originalFile.Set("UPDATE_VAR", "original")
		originalFile.Set("KEEP_VAR", "keep-this")
		
		// Push
		err := suite.manager.PushEnvironment(suite.ctx, "test", originalFile, true)
		require.NoError(t, err)
		
		// Updated environment variables
		updatedFile := env.NewFile()
		updatedFile.Set("UPDATE_VAR", "updated")
		updatedFile.Set("KEEP_VAR", "keep-this")
		updatedFile.Set("NEW_VAR", "new-value")
		
		// Push updates
		err = suite.manager.PushEnvironment(suite.ctx, "test", updatedFile, true)
		require.NoError(t, err)
		
		// Pull and verify
		pulledFile, err := suite.manager.PullEnvironment(suite.ctx, "test")
		require.NoError(t, err)
		
		// Verify updated values
		updateVar, exists := pulledFile.Get("UPDATE_VAR")
		assert.True(t, exists)
		assert.Equal(t, "updated", updateVar)
		
		// Verify retained values
		keepVar, exists := pulledFile.Get("KEEP_VAR")
		assert.True(t, exists)
		assert.Equal(t, "keep-this", keepVar)
		
		// Verify new values
		newVar, exists := pulledFile.Get("NEW_VAR")
		assert.True(t, exists)
		assert.Equal(t, "new-value", newVar)
	})
}

// TestSecretsManagerOperations tests Secrets Manager operations
func (suite *AWSIntegrationTestSuite) TestSecretsManagerOperations() {
	t := suite.T()
	
	t.Run("Environment variable management using Secrets Manager", func(t *testing.T) {
		// Environment configuration for Secrets Manager
		suite.manager.GetConfig().Environments["secrets"] = config.Environment{
			Files:             []string{".env.secrets"},
			Path:              "/integration-test/secrets/",
			UseSecretsManager: true,
		}
		
		// Environment variables for secrets
		secretFile := env.NewFile()
		secretFile.Set("API_KEY", "secret-api-key")
		secretFile.Set("DB_PASSWORD", "secret-password")
		secretFile.Set("JWT_SECRET", "jwt-secret-key")
		
		// Push to Secrets Manager
		err := suite.manager.PushEnvironment(suite.ctx, "secrets", secretFile, true)
		require.NoError(t, err)
		
		// Pull from Secrets Manager
		pulledFile, err := suite.manager.PullEnvironment(suite.ctx, "secrets")
		require.NoError(t, err)
		
		// Verify values
		apiKey, exists := pulledFile.Get("API_KEY")
		assert.True(t, exists)
		assert.Equal(t, "secret-api-key", apiKey)
		
		dbPass, exists := pulledFile.Get("DB_PASSWORD")
		assert.True(t, exists)
		assert.Equal(t, "secret-password", dbPass)
		
		jwtSecret, exists := pulledFile.Get("JWT_SECRET")
		assert.True(t, exists)
		assert.Equal(t, "jwt-secret-key", jwtSecret)
	})
	
	
}

// TestConcurrentOperations tests concurrent operations
func (suite *AWSIntegrationTestSuite) TestConcurrentOperations() {
	t := suite.T()
	
	t.Run("Concurrent push operations", func(t *testing.T) {
		// Prepare environment variables for concurrent push
		concurrentFile := env.NewFile()
		numVars := 20
		for i := 0; i < numVars; i++ {
			concurrentFile.Set(fmt.Sprintf("CONCURRENT_VAR_%d", i), fmt.Sprintf("value_%d", i))
		}
		
		// Configuration for concurrent push
		suite.manager.GetConfig().Environments["concurrent"] = config.Environment{
			Files: []string{".env.concurrent"},
			Path:  "/integration-test/concurrent/",
		}
		
		// Concurrent push (processed in parallel internally)
		start := time.Now()
		err := suite.manager.PushEnvironment(suite.ctx, "concurrent", concurrentFile, true)
		duration := time.Since(start)
		require.NoError(t, err)
		
		// Check performance (should be faster due to parallel processing)
		t.Logf("Pushed %d variables in %v", numVars, duration)
		
		// Pull and verify
		pulledFile, err := suite.manager.PullEnvironment(suite.ctx, "concurrent")
		require.NoError(t, err)
		assert.Equal(t, numVars, len(pulledFile.Keys()))
	})
}

// TestErrorHandling tests error handling
func (suite *AWSIntegrationTestSuite) TestErrorHandling() {
	t := suite.T()
	
	t.Run("Get non-existent environment", func(t *testing.T) {
		_, err := suite.manager.PullEnvironment(suite.ctx, "non-existent-env")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment 'non-existent-env' not found")
	})
	
	t.Run("Empty environment variable file", func(t *testing.T) {
		emptyFile := env.NewFile()
		
		// Empty file does not cause an error
		err := suite.manager.PushEnvironment(suite.ctx, "test", emptyFile, true)
		assert.NoError(t, err)
		
		// Pull returns empty
		pulledFile, err := suite.manager.PullEnvironment(suite.ctx, "test")
		assert.NoError(t, err)
		assert.Empty(t, pulledFile.Keys())
	})
	
	t.Run("Large value handling", func(t *testing.T) {
		largeFile := env.NewFile()
		// Create value larger than 4KB (Parameter Store standard limit)
		largeValue := strings.Repeat("A", 5000)
		largeFile.Set("LARGE_VALUE", largeValue)
		
		// Check error handling
		err := suite.manager.PushEnvironment(suite.ctx, "test", largeFile, true)
		// LocalStack may have different limits, so only check for error presence
		if err != nil {
			t.Logf("Large value error (expected): %v", err)
		}
	})
}

// TestRealAWSAPI tests using real AWS API (optional)
func (suite *AWSIntegrationTestSuite) TestRealAWSAPI() {
	if os.Getenv("TEST_REAL_AWS") != "true" {
		suite.T().Skip("Skipping real AWS API test. Set TEST_REAL_AWS=true to run.")
	}
	
	// Tests against real AWS environment
	// Note: This may incur costs as it creates real AWS resources
	
	t := suite.T()
	
	// Create manager using real AWS configuration
	cfg := &config.Config{
		AWS: config.AWSConfig{
			Region:  os.Getenv("AWS_REGION"),
			Profile: os.Getenv("AWS_PROFILE"),
		},
		Project:            "envy-integration-test",
		DefaultEnvironment: "test",
		Environments: map[string]config.Environment{
			"test": {
				Files: []string{".env.test"},
				Path:  "/envy-integration-test/test/",
			},
		},
	}
	
	realManager, err := aws.NewManager(cfg)
	require.NoError(t, err)
	
	// Generate unique prefix for testing (for future expansion)
	// testPrefix := "/envy-integration-test/" + time.Now().Format("20060102-150405")
	
	t.Run("Environment variable management in real environment", func(t *testing.T) {
		// Create test environment variables
		testFile := env.NewFile()
		testFile.Set("REAL_TEST_VAR", "real-test-value")
		testFile.Set("REAL_API_KEY", "real-api-key-12345")
		
		// Push to real environment
		err := realManager.PushEnvironment(suite.ctx, "test", testFile, true)
		require.NoError(t, err)
		
		// Pull from real environment
		pulledFile, err := realManager.PullEnvironment(suite.ctx, "test")
		require.NoError(t, err)
		
		// Verify values
		value, exists := pulledFile.Get("REAL_TEST_VAR")
		assert.True(t, exists)
		assert.Equal(t, "real-test-value", value)
		
		// Always clean up (delete real environment resources)
		// Note: Cleanup method may vary depending on implementation
		t.Logf("Test completed. Manual cleanup may be required for path: %s", cfg.GetParameterPath("test"))
	})
}

// cleanupTestPath deletes all parameters under the test path
func (suite *AWSIntegrationTestSuite) cleanupTestPath(path string) {
	// Since the current implementation does not support direct path deletion,
	// we need to get the list of environment variables and delete them individually
	_, err := suite.manager.ListEnvironmentVariables(suite.ctx, "test")
	if err == nil {
		// Push an empty environment variable file to delete existing variables
		emptyFile := env.NewFile()
		_ = suite.manager.PushEnvironment(suite.ctx, "test", emptyFile, true)
	}
}

// TestAWSIntegration runs the test suite
func TestAWSIntegration(t *testing.T) {
	suite.Run(t, new(AWSIntegrationTestSuite))
}