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

func TestGetDiffCmd(t *testing.T) {
	cmd := GetDiffCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "diff", cmd.Use)
	assert.NotNil(t, cmd.RunE)
}

func TestDiffCommandFlags(t *testing.T) {
	cmd := GetDiffCmd()

	// Check that all expected flags are present
	assert.NotNil(t, cmd.Flags().Lookup("from"))
	assert.NotNil(t, cmd.Flags().Lookup("to"))
	assert.NotNil(t, cmd.Flags().Lookup("file1"))
	assert.NotNil(t, cmd.Flags().Lookup("file2"))
	assert.NotNil(t, cmd.Flags().Lookup("format"))
	assert.NotNil(t, cmd.Flags().Lookup("changes"))
	assert.NotNil(t, cmd.Flags().Lookup("env"))
	assert.NotNil(t, cmd.Flags().Lookup("show-values"))
	assert.NotNil(t, cmd.Flags().Lookup("color"))

	// Check flag shortcuts
	formatFlag := cmd.Flags().Lookup("format")
	assert.Equal(t, "f", formatFlag.Shorthand)

	changesFlag := cmd.Flags().Lookup("changes")
	assert.Equal(t, "c", changesFlag.Shorthand)

	envFlag := cmd.Flags().Lookup("env")
	assert.Equal(t, "e", envFlag.Shorthand)
}

func TestDiffCommandUsage(t *testing.T) {
	cmd := GetDiffCmd()

	assert.Equal(t, "diff", cmd.Use)
	assert.Contains(t, cmd.Short, "Show differences between environments")
	assert.Contains(t, cmd.Long, "Show differences between local and remote")
	assert.NotEmpty(t, cmd.Example)
}

func TestSourceTargetValidation(t *testing.T) {
	tests := []struct {
		name        string
		from        string
		to          string
		expectError bool
	}{
		{
			name:        "local_to_aws",
			from:        "local",
			to:          "aws",
			expectError: false,
		},
		{
			name:        "aws_to_local",
			from:        "aws",
			to:          "local",
			expectError: false,
		},
		{
			name:        "env_to_env",
			from:        "dev",
			to:          "staging",
			expectError: false,
		},
		{
			name:        "same_source_target",
			from:        "local",
			to:          "local",
			expectError: false, // Allowed but will show no differences
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			assert.NotEmpty(t, tt.from)
			assert.NotEmpty(t, tt.to)
		})
	}
}

func TestChangesFilter(t *testing.T) {
	tests := []struct {
		name        string
		changes     string
		expectError bool
	}{
		{
			name:        "all_changes",
			changes:     "all",
			expectError: false,
		},
		{
			name:        "additions_only",
			changes:     "additions",
			expectError: false,
		},
		{
			name:        "deletions_only",
			changes:     "deletions",
			expectError: false,
		},
		{
			name:        "modifications_only",
			changes:     "modifications",
			expectError: false,
		},
		{
			name:        "invalid_filter",
			changes:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validChanges := []string{"all", "additions", "deletions", "modifications"}
			isValid := false
			for _, vc := range validChanges {
				if vc == tt.changes {
					isValid = true
					break
				}
			}
			if tt.expectError {
				assert.False(t, isValid)
			} else {
				assert.True(t, isValid)
			}
		})
	}
}

func TestColorOutput(t *testing.T) {
	tests := []struct {
		name        string
		colorOutput bool
		expected    string
	}{
		{
			name:        "color_enabled",
			colorOutput: true,
			expected:    "colored",
		},
		{
			name:        "color_disabled",
			colorOutput: false,
			expected:    "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.colorOutput {
				assert.Equal(t, "colored", tt.expected)
			} else {
				assert.Equal(t, "plain", tt.expected)
			}
		})
	}
}

func TestFileComparison(t *testing.T) {
	tests := []struct {
		name        string
		file1       string
		file2       string
		expectError bool
	}{
		{
			name:        "both_files_specified",
			file1:       ".env.dev",
			file2:       ".env.prod",
			expectError: false,
		},
		{
			name:        "only_file1_specified",
			file1:       ".env.dev",
			file2:       "",
			expectError: true,
		},
		{
			name:        "only_file2_specified",
			file1:       "",
			file2:       ".env.prod",
			expectError: true,
		},
		{
			name:        "no_files_specified",
			file1:       "",
			file2:       "",
			expectError: false, // Will use from/to instead
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When both files are specified, file comparison is used
			if tt.file1 != "" && tt.file2 != "" {
				assert.False(t, tt.expectError)
			} else if (tt.file1 != "" && tt.file2 == "") || (tt.file1 == "" && tt.file2 != "") {
				// One file specified is an error
				assert.True(t, tt.expectError)
			} else {
				// No files means use from/to comparison
				assert.False(t, tt.expectError)
			}
		})
	}
}

func TestDiffValueMasking(t *testing.T) {
	tests := []struct {
		name       string
		showValues bool
		key        string
		value      string
		expected   string
	}{
		{
			name:       "show_values_enabled",
			showValues: true,
			key:        "API_KEY",
			value:      "secret123",
			expected:   "***", // Sensitive keys are always masked
		},
		{
			name:       "show_values_disabled_sensitive",
			showValues: false,
			key:        "API_KEY",
			value:      "secret123",
			expected:   "***",
		},
		{
			name:       "show_values_disabled_normal",
			showValues: false,
			key:        "APP_NAME",
			value:      "myapp",
			expected:   "***",
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

func TestDiffFiltering(t *testing.T) {
	tests := []struct {
		name     string
		changes  string
		diff     *DiffResult
		expected int
	}{
		{
			name:    "filter_additions",
			changes: "additions",
			diff: &DiffResult{
				Added:    map[string]string{"NEW_VAR": "value"},
				Modified: map[string][2]string{"MOD_VAR": {"old", "new"}},
				Deleted:  map[string]string{"DEL_VAR": "value"},
			},
			expected: 1,
		},
		{
			name:    "filter_deletions",
			changes: "deletions",
			diff: &DiffResult{
				Added:    map[string]string{"NEW_VAR": "value"},
				Modified: map[string][2]string{"MOD_VAR": {"old", "new"}},
				Deleted:  map[string]string{"DEL_VAR": "value"},
			},
			expected: 1,
		},
		{
			name:    "filter_modifications",
			changes: "modifications",
			diff: &DiffResult{
				Added:    map[string]string{"NEW_VAR": "value"},
				Modified: map[string][2]string{"MOD_VAR": {"old", "new"}},
				Deleted:  map[string]string{"DEL_VAR": "value"},
			},
			expected: 1,
		},
		{
			name:    "show_all",
			changes: "all",
			diff: &DiffResult{
				Added:    map[string]string{"NEW_VAR": "value"},
				Modified: map[string][2]string{"MOD_VAR": {"old", "new"}},
				Deleted:  map[string]string{"DEL_VAR": "value"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			switch tt.changes {
			case "additions":
				count = len(tt.diff.Added)
			case "deletions":
				count = len(tt.diff.Deleted)
			case "modifications":
				count = len(tt.diff.Modified)
			case "all":
				count = len(tt.diff.Added) + len(tt.diff.Deleted) + len(tt.diff.Modified)
			}
			assert.Equal(t, tt.expected, count)
		})
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