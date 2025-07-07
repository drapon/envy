package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSensitive(t *testing.T) {
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
			name: "secret key",
			key:  "SECRET_KEY",
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
			name: "credentials",
			key:  "AWS_CREDENTIALS",
			want: true,
		},
		{
			name: "private key",
			key:  "PRIVATE_KEY",
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
			name: "case insensitive",
			key:  "password",
			want: true,
		},
		{
			name: "partial match",
			key:  "DB_PASSWORD",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitive(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "short value",
			value: "abc",
			want:  "****",
		},
		{
			name:  "exact 4 chars",
			value: "1234",
			want:  "****",
		},
		{
			name:  "long value",
			value: "secret123",
			want:  "se*****23",
		},
		{
			name:  "very long value",
			value: "verylongsecretvalue",
			want:  "ve***************ue",
		},
		{
			name:  "empty value",
			value: "",
			want:  "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunCommand(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}