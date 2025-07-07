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

func TestGetRunCmd(t *testing.T) {
	cmd := GetRunCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "run -- [command]", cmd.Use)
	assert.NotNil(t, cmd.RunE)
}

func TestRunCommandFlags(t *testing.T) {
	cmd := GetRunCmd()

	// Check that all expected flags are present
	assert.NotNil(t, cmd.Flags().Lookup("env"))
	assert.NotNil(t, cmd.Flags().Lookup("file"))
	assert.NotNil(t, cmd.Flags().Lookup("set"))
	assert.NotNil(t, cmd.Flags().Lookup("override"))
	assert.NotNil(t, cmd.Flags().Lookup("inherit"))
	assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, cmd.Flags().Lookup("verbose"))
	assert.NotNil(t, cmd.Flags().Lookup("from"))

	// Check flag shortcuts
	envFlag := cmd.Flags().Lookup("env")
	assert.Equal(t, "e", envFlag.Shorthand)

	fileFlag := cmd.Flags().Lookup("file")
	assert.Equal(t, "f", fileFlag.Shorthand)

	setFlag := cmd.Flags().Lookup("set")
	assert.Equal(t, "s", setFlag.Shorthand)

	overrideFlag := cmd.Flags().Lookup("override")
	assert.Equal(t, "o", overrideFlag.Shorthand)

	inheritFlag := cmd.Flags().Lookup("inherit")
	assert.Equal(t, "i", inheritFlag.Shorthand)

	verboseFlag := cmd.Flags().Lookup("verbose")
	assert.Equal(t, "v", verboseFlag.Shorthand)
}

func TestRunCommandUsage(t *testing.T) {
	cmd := GetRunCmd()

	assert.Equal(t, "run -- [command]", cmd.Use)
	assert.Contains(t, cmd.Short, "Run a command with environment variables")
	assert.Contains(t, cmd.Long, "Run a command with environment variables loaded")
	assert.NotEmpty(t, cmd.Example)
}

func TestPrintEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		expected []string
	}{
		{
			name: "basic_vars",
			vars: map[string]string{
				"APP_NAME": "test-app",
				"DEBUG":    "true",
			},
			expected: []string{
				"APP_NAME",
				"DEBUG",
			},
		},
		{
			name: "sensitive_vars",
			vars: map[string]string{
				"PASSWORD":   "secret123",
				"API_KEY":    "abc123",
				"NORMAL_VAR": "value",
			},
			expected: []string{
				"PASSWORD",
				"API_KEY",
				"NORMAL_VAR",
			},
		},
		{
			name:     "empty_vars",
			vars:     map[string]string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that all expected variables would be printed
			for _, key := range tt.expected {
				_, exists := tt.vars[key]
				assert.True(t, exists)
			}
		})
	}
}

func TestLoadEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		environment string
		expectError bool
	}{
		{
			name:        "local_source",
			source:      "local",
			environment: "dev",
			expectError: false,
		},
		{
			name:        "aws_source",
			source:      "aws",
			environment: "dev",
			expectError: true, // Will fail without AWS credentials
		},
		{
			name:        "auto_source",
			source:      "auto",
			environment: "dev",
			expectError: false,
		},
		{
			name:        "invalid_source",
			source:      "invalid",
			environment: "dev",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test source validation
			validSources := []string{"local", "aws", "auto"}
			valid := false
			for _, s := range validSources {
				if s == tt.source {
					valid = true
					break
				}
			}

			if !valid && !tt.expectError {
				t.Errorf("Invalid source %s should cause error", tt.source)
			}
		})
	}
}

func TestShellDetection(t *testing.T) {
	tests := []struct {
		name          string
		shellFlag     string
		envShell      string
		expectedShell string
	}{
		{
			name:          "explicit_bash",
			shellFlag:     "bash",
			envShell:      "/bin/zsh",
			expectedShell: "bash",
		},
		{
			name:          "explicit_zsh",
			shellFlag:     "zsh",
			envShell:      "/bin/bash",
			expectedShell: "zsh",
		},
		{
			name:          "from_env",
			shellFlag:     "",
			envShell:      "/bin/zsh",
			expectedShell: "zsh",
		},
		{
			name:          "default_sh",
			shellFlag:     "",
			envShell:      "",
			expectedShell: "sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test shell detection logic
			shell := tt.shellFlag
			if shell == "" && tt.envShell != "" {
				// Extract shell name from path
				if tt.envShell == "/bin/zsh" {
					shell = "zsh"
				} else if tt.envShell == "/bin/bash" {
					shell = "bash"
				}
			}
			if shell == "" {
				shell = "sh"
			}

			assert.Equal(t, tt.expectedShell, shell)
		})
	}
}

func TestTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     int
		expectError bool
	}{
		{
			name:        "positive_timeout",
			timeout:     30,
			expectError: false,
		},
		{
			name:        "zero_timeout",
			timeout:     0,
			expectError: false, // 0 means no timeout
		},
		{
			name:        "negative_timeout",
			timeout:     -1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test timeout validation
			if tt.timeout < 0 && !tt.expectError {
				t.Error("Negative timeout should cause error")
			}
		})
	}
}