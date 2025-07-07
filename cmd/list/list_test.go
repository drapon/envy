package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		filter string
		want   bool
	}{
		{
			name:   "empty filter matches all",
			key:    "KEY1",
			filter: "",
			want:   true,
		},
		{
			name:   "exact match",
			key:    "DB_HOST",
			filter: "DB_",
			want:   true,
		},
		{
			name:   "case insensitive match",
			key:    "db_host",
			filter: "DB_",
			want:   true,
		},
		{
			name:   "no match",
			key:    "APP_NAME",
			filter: "DB_",
			want:   false,
		},
		{
			name:   "contains match",
			key:    "MY_DB_HOST",
			filter: "DB",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilter(tt.key, tt.filter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		showValues bool
		want       string
	}{
		{
			name:       "show values enabled",
			key:        "KEY1",
			value:      "value1",
			showValues: true,
			want:       "value1",
		},
		{
			name:       "sensitive key with show values disabled",
			key:        "PASSWORD",
			value:      "secret123",
			showValues: false,
			want:       "****",
		},
		{
			name:       "non-sensitive key with show values disabled",
			key:        "APP_NAME",
			value:      "myapp",
			showValues: false,
			want:       "myapp",
		},
		{
			name:       "API_KEY with show values disabled",
			key:        "API_KEY",
			value:      "abc123xyz",
			showValues: false,
			want:       "****",
		},
		{
			name:       "empty value",
			key:        "EMPTY_KEY",
			value:      "",
			showValues: false,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			showValues = tt.showValues
			got := maskValue(tt.key, tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "password key",
			key:  "PASSWORD",
			want: true,
		},
		{
			name: "db password",
			key:  "DB_PASSWORD",
			want: true,
		},
		{
			name: "secret key",
			key:  "APP_SECRET",
			want: true,
		},
		{
			name: "API key",
			key:  "API_KEY",
			want: true,
		},
		{
			name: "token",
			key:  "AUTH_TOKEN",
			want: true,
		},
		{
			name: "private key",
			key:  "PRIVATE_KEY",
			want: true,
		},
		{
			name: "credentials",
			key:  "AWS_CREDENTIALS",
			want: true,
		},
		{
			name: "non-sensitive key",
			key:  "APP_NAME",
			want: false,
		},
		{
			name: "database host",
			key:  "DB_HOST",
			want: false,
		},
		{
			name: "port number",
			key:  "APP_PORT",
			want: false,
		},
		{
			name: "case insensitive password",
			key:  "password",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitiveKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListCommand(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}