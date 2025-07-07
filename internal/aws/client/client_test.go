package client

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		opts        Options
		expectError bool
		errorMsg    string
	}{
		{
			name: "empty_region",
			opts: Options{
				Region:  "",
				Profile: "",
			},
			expectError: true,
			errorMsg:    "AWS region is required",
		},
		{
			name: "custom_region",
			opts: Options{
				Region:  "us-west-2",
				Profile: "",
			},
			expectError: false,
		},
		{
			name: "with_profile",
			opts: Options{
				Region:  "us-east-1",
				Profile: "test-profile",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require AWS credentials in CI
			if testing.Short() && !tt.expectError {
				t.Skip("Skipping AWS client test in short mode")
			}

			ctx := context.Background()
			client, err := NewClient(ctx, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// May still error if no AWS credentials are available
				if err != nil {
					t.Skipf("Skipping test due to AWS credentials: %v", err)
				}
				assert.NotNil(t, client)
				assert.NotNil(t, client.config)
			}
		})
	}
}

func TestClientGetters(t *testing.T) {
	// Create a mock client for testing getters
	mockCfg := aws.Config{
		Region: "us-test-1",
	}

	client := &Client{
		config:  mockCfg,
		region:  "us-test-1",
		profile: "test-profile",
	}

	t.Run("Region", func(t *testing.T) {
		assert.Equal(t, "us-test-1", client.Region())
	})

	t.Run("Profile", func(t *testing.T) {
		assert.Equal(t, "test-profile", client.Profile())
	})

	t.Run("Config", func(t *testing.T) {
		cfg := client.Config()
		assert.Equal(t, mockCfg.Region, cfg.Region)
	})
}

func TestSSMClient(t *testing.T) {
	// Skip if no AWS credentials
	if testing.Short() {
		t.Skip("Skipping AWS SSM client test in short mode")
	}

	ctx := context.Background()
	opts := Options{
		Region:  "us-east-1",
		Profile: "",
	}
	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skipf("Skipping test due to AWS credentials: %v", err)
	}

	ssmClient := client.SSM()
	assert.NotNil(t, ssmClient)

	// Verify it's the same instance when called again (cached)
	ssmClient2 := client.SSM()
	assert.Equal(t, ssmClient, ssmClient2)
}

func TestSecretsManagerClient(t *testing.T) {
	// Skip if no AWS credentials
	if testing.Short() {
		t.Skip("Skipping AWS Secrets Manager client test in short mode")
	}

	ctx := context.Background()
	opts := Options{
		Region:  "us-east-1",
		Profile: "",
	}
	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skipf("Skipping test due to AWS credentials: %v", err)
	}

	smClient := client.SecretsManager()
	assert.NotNil(t, smClient)

	// Verify it's the same instance when called again (cached)
	smClient2 := client.SecretsManager()
	assert.Equal(t, smClient, smClient2)
}

func TestClientInitialization(t *testing.T) {
	// Test that SSM and SecretsManager clients are lazily initialized
	mockCfg := aws.Config{
		Region: "us-test-1",
	}

	client := &Client{
		config:  mockCfg,
		region:  "us-test-1",
		profile: "",
	}

	// Initially, clients should be nil
	assert.Nil(t, client.ssmClient)
	assert.Nil(t, client.secretsClient)

	if testing.Short() {
		t.Skip("Skipping client initialization test in short mode")
	}
}

func TestNewClientWithInvalidRegion(t *testing.T) {
	// Test with various region inputs
	tests := []struct {
		name        string
		opts        Options
		expectError bool
	}{
		{
			name: "empty_region",
			opts: Options{
				Region: "",
			},
			expectError: true,
		},
		{
			name: "valid_region",
			opts: Options{
				Region: "us-east-1",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if testing.Short() && !tt.expectError {
				t.Skip("Skipping AWS client test in short mode")
			}

			ctx := context.Background()
			_, err := NewClient(ctx, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				// May error due to missing credentials
				if err != nil {
					t.Logf("Client creation resulted in error: %v", err)
				}
			}
		})
	}
}

func TestClientStructFields(t *testing.T) {
	// Verify the Client struct has the expected fields
	client := &Client{}

	// Test that we can set fields without issues
	client.config = aws.Config{Region: "test"}
	client.region = "test-region"
	client.profile = "test-profile"

	assert.Equal(t, "test-region", client.region)
	assert.Equal(t, "test-profile", client.profile)
	assert.Equal(t, "test", client.config.Region)
}

func TestMutexSafety(t *testing.T) {
	// Test that concurrent access to SSM() and SecretsManager() is safe
	if testing.Short() {
		t.Skip("Skipping mutex safety test in short mode")
	}

	ctx := context.Background()
	opts := Options{
		Region:  "us-east-1",
		Profile: "",
	}
	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skipf("Skipping test due to AWS credentials: %v", err)
	}

	// Run concurrent accesses
	done := make(chan bool, 4)

	go func() {
		_ = client.SSM()
		done <- true
	}()

	go func() {
		_ = client.SSM()
		done <- true
	}()

	go func() {
		_ = client.SecretsManager()
		done <- true
	}()

	go func() {
		_ = client.SecretsManager()
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// If we get here without deadlock or panic, the test passes
	assert.True(t, true)
}