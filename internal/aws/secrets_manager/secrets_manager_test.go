package secrets_manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatEnvKey(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		expected   string
	}{
		{
			name:       "simple name",
			secretName: "myapp/dev/database",
			expected:   "MYAPP_DEV_DATABASE",
		},
		{
			name:       "with hyphens",
			secretName: "myapp/dev/api-key",
			expected:   "MYAPP_DEV_API_KEY",
		},
		{
			name:       "with multiple slashes",
			secretName: "myapp/dev/services/auth/token",
			expected:   "MYAPP_DEV_SERVICES_AUTH_TOKEN",
		},
		{
			name:       "already uppercase",
			secretName: "myapp/dev/API_KEY",
			expected:   "MYAPP_DEV_API_KEY",
		},
		{
			name:       "with dots",
			secretName: "myapp/dev/config.file",
			expected:   "MYAPP_DEV_CONFIG_FILE",
		},
		{
			name:       "single part",
			secretName: "secret",
			expected:   "SECRET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEnvKey(tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewManager(t *testing.T) {
	// Skip this test as it requires actual AWS client
	t.Skip("Skipping test that requires AWS client")
}