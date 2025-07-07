package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffCommand(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

func TestSortedKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected []string
	}{
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: []string{},
		},
		{
			name: "single key",
			input: map[string]string{
				"KEY1": "value1",
			},
			expected: []string{"KEY1"},
		},
		{
			name: "multiple keys",
			input: map[string]string{
				"KEY3": "value3",
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{"KEY1", "KEY2", "KEY3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sortedKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		showValues bool
		expected   string
	}{
		{
			name:       "show values enabled for non-sensitive key",
			key:        "APP_NAME",
			value:      "myapp",
			showValues: true,
			expected:   "myapp",
		},
		{
			name:       "sensitive key with show values disabled",
			key:        "PASSWORD",
			value:      "secret123",
			showValues: false,
			expected:   "***",
		},
		{
			name:       "non-sensitive key with show values disabled",
			key:        "APP_NAME",
			value:      "myapp",
			showValues: false,
			expected:   "***",
		},
		{
			name:       "API_KEY with show values disabled",
			key:        "API_KEY",
			value:      "abc123xyz",
			showValues: false,
			expected:   "***",
		},
		{
			name:       "sensitive key with show values enabled",
			key:        "PASSWORD",
			value:      "secret123",
			showValues: true,
			expected:   "***",  // Still masked because it's sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			showValues = tt.showValues
			result := maskValue(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "password key",
			key:      "PASSWORD",
			expected: true,
		},
		{
			name:     "secret key",
			key:      "APP_SECRET",
			expected: true,
		},
		{
			name:     "API key",
			key:      "API_KEY",
			expected: true,
		},
		{
			name:     "token",
			key:      "AUTH_TOKEN",
			expected: true,
		},
		{
			name:     "private key",
			key:      "PRIVATE_KEY",
			expected: true,
		},
		{
			name:     "non-sensitive key",
			key:      "APP_NAME",
			expected: false,
		},
		{
			name:     "database host",
			key:      "DB_HOST",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateDiff(t *testing.T) {
	tests := []struct {
		name     string
		vars1    map[string]string
		vars2    map[string]string
		expected *DiffResult
	}{
		{
			name:  "no differences",
			vars1: map[string]string{"KEY1": "value1"},
			vars2: map[string]string{"KEY1": "value1"},
			expected: &DiffResult{
				Added:     map[string]string{},
				Deleted:   map[string]string{},
				Modified:  map[string][2]string{},
				Unchanged: map[string]string{"KEY1": "value1"},
			},
		},
		{
			name:  "additions only",
			vars1: map[string]string{"KEY1": "value1"},
			vars2: map[string]string{"KEY1": "value1", "KEY2": "value2"},
			expected: &DiffResult{
				Added:     map[string]string{"KEY2": "value2"},
				Deleted:   map[string]string{},
				Modified:  map[string][2]string{},
				Unchanged: map[string]string{"KEY1": "value1"},
			},
		},
		{
			name:  "deletions only",
			vars1: map[string]string{"KEY1": "value1", "KEY2": "value2"},
			vars2: map[string]string{"KEY1": "value1"},
			expected: &DiffResult{
				Added:    map[string]string{},
				Deleted:  map[string]string{"KEY2": "value2"},
				Modified: map[string][2]string{},
				Unchanged: map[string]string{"KEY1": "value1"},
			},
		},
		{
			name:  "modifications",
			vars1: map[string]string{"KEY1": "value1"},
			vars2: map[string]string{"KEY1": "value2"},
			expected: &DiffResult{
				Added:    map[string]string{},
				Deleted:  map[string]string{},
				Modified: map[string][2]string{
					"KEY1": {"value1", "value2"},
				},
				Unchanged: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateDiff(tt.vars1, tt.vars2)
			assert.Equal(t, tt.expected, result)
		})
	}
}