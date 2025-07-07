package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drapon/envy/internal/env"
	"github.com/stretchr/testify/assert"
)

func TestExportShell(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		expected []string
	}{
		{
			name: "basic export",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{
				"export KEY1='value1'",
				"export KEY2='value2'",
			},
		},
		{
			name: "values with spaces",
			vars: map[string]string{
				"KEY1": "value with spaces",
			},
			expected: []string{
				"export KEY1='value with spaces'",
			},
		},
		{
			name: "values with single quotes",
			vars: map[string]string{
				"KEY1": "value's",
			},
			expected: []string{
				"export KEY1='value'\"'\"'s'",
			},
		},
		{
			name:     "empty vars",
			vars:     map[string]string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create env.File from map
			envFile := env.NewFile()
			for k, v := range tt.vars {
				envFile.Set(k, v)
			}

			buf := new(bytes.Buffer)
			err := exportShell(buf, envFile)
			assert.NoError(t, err)

			output := buf.String()
			for _, exp := range tt.expected {
				assert.Contains(t, output, exp)
			}
		})
	}
}

func TestExportDocker(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		expected []string
	}{
		{
			name: "basic export",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{
				"KEY1=value1",
				"KEY2=value2",
			},
		},
		{
			name: "values with spaces",
			vars: map[string]string{
				"KEY1": "value with spaces",
			},
			expected: []string{
				"KEY1=value with spaces",
			},
		},
		{
			name: "values with equals",
			vars: map[string]string{
				"KEY1": "value=123",
			},
			expected: []string{
				"KEY1=value=123",
			},
		},
		{
			name:     "empty vars",
			vars:     map[string]string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create env.File from map
			envFile := env.NewFile()
			for k, v := range tt.vars {
				envFile.Set(k, v)
			}

			buf := new(bytes.Buffer)
			err := exportDocker(buf, envFile)
			assert.NoError(t, err)

			output := buf.String()
			for _, exp := range tt.expected {
				assert.Contains(t, output, exp)
			}
		})
	}
}

func TestExportJSON(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "basic export",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			checkFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, `"KEY1": "value1"`)
				assert.Contains(t, output, `"KEY2": "value2"`)
				assert.True(t, strings.HasPrefix(output, "{"))
				assert.True(t, strings.HasSuffix(strings.TrimSpace(output), "}"))
			},
		},
		{
			name: "special characters",
			vars: map[string]string{
				"KEY1": "value\"with\"quotes",
			},
			checkFunc: func(t *testing.T, output string) {
				assert.Contains(t, output, `"KEY1": "value\"with\"quotes"`)
			},
		},
		{
			name: "empty vars",
			vars: map[string]string{},
			checkFunc: func(t *testing.T, output string) {
				assert.Equal(t, "{}\n", output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create env.File from map
			envFile := env.NewFile()
			for k, v := range tt.vars {
				envFile.Set(k, v)
			}

			buf := new(bytes.Buffer)
			err := exportJSON(buf, envFile)
			assert.NoError(t, err)

			if tt.checkFunc != nil {
				tt.checkFunc(t, buf.String())
			}
		})
	}
}

func TestExportYAML(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		expected []string
	}{
		{
			name: "basic export",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{
				"KEY1: value1",
				"KEY2: value2",
			},
		},
		{
			name: "values needing quotes",
			vars: map[string]string{
				"KEY1": "value: with colon",
			},
			expected: []string{
				"KEY1: 'value: with colon'",
			},
		},
		{
			name:     "empty vars",
			vars:     map[string]string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create env.File from map
			envFile := env.NewFile()
			for k, v := range tt.vars {
				envFile.Set(k, v)
			}

			buf := new(bytes.Buffer)
			err := exportYAML(buf, envFile)
			assert.NoError(t, err)

			output := buf.String()
			for _, exp := range tt.expected {
				assert.Contains(t, output, exp)
			}
		})
	}
}

func TestApplyFilters(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		filter   string
		exclude  string
		expected map[string]string
	}{
		{
			name: "no filters",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			filter:  "",
			exclude: "",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		},
		{
			name: "include filter",
			vars: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
				"APP_NAME": "myapp",
			},
			filter:  "^DB_",
			exclude: "",
			expected: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
		},
		{
			name: "exclude filter",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"SECRET_KEY": "secret",
			},
			filter:  "",
			exclude: "SECRET",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		},
		{
			name: "both filters",
			vars: map[string]string{
				"DB_HOST": "localhost",
				"DB_PASSWORD": "secret",
				"APP_NAME": "myapp",
			},
			filter:  "^DB_",
			exclude: "PASSWORD",
			expected: map[string]string{
				"DB_HOST": "localhost",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create env.File from map
			envFile := env.NewFile()
			for k, v := range tt.vars {
				envFile.Set(k, v)
			}

			result := applyFilters(envFile, tt.filter, tt.exclude)
			
			// Convert result to map for comparison
			resultMap := result.ToMap()
			assert.Equal(t, tt.expected, resultMap)
		})
	}
}

func TestExportCommand(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}