package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	baseDir := "/tmp/test"
	manager := NewManager(baseDir)
	
	assert.NotNil(t, manager)
	assert.Equal(t, baseDir, manager.basePath)
}

func TestLoadFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create a test .env file
	envContent := `APP_NAME=test-app
DEBUG=true
# Comment line
PORT=8080
`
	envFile := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envFile, []byte(envContent), 0644)
	require.NoError(t, err)

	// Test loading the file
	file, err := manager.LoadFile(".env")
	assert.NoError(t, err)
	assert.NotNil(t, file)
	assert.Equal(t, 3, len(file.Variables))
	assert.Equal(t, "test-app", file.Variables["APP_NAME"].Value)
	assert.Equal(t, "true", file.Variables["DEBUG"].Value)
	assert.Equal(t, "8080", file.Variables["PORT"].Value)
}

func TestLoadFiles(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create test .env files
	files := map[string]string{
		".env":      "APP_NAME=base\nPORT=8080",
		".env.dev":  "APP_NAME=dev\nDEBUG=true",
		".env.prod": "APP_NAME=prod\nDEBUG=false\nSECRET=prod-secret",
	}

	for filename, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
		require.NoError(t, err)
	}

	tests := []struct {
		name     string
		files    []string
		expected map[string]string
	}{
		{
			name:  "single file",
			files: []string{".env"},
			expected: map[string]string{
				"APP_NAME": "base",
				"PORT":     "8080",
			},
		},
		{
			name:  "multiple files with override",
			files: []string{".env", ".env.dev"},
			expected: map[string]string{
				"APP_NAME": "dev",  // Overridden
				"PORT":     "8080", // From base
				"DEBUG":    "true", // From dev
			},
		},
		{
			name:  "all files",
			files: []string{".env", ".env.prod"},
			expected: map[string]string{
				"APP_NAME": "prod",        // Overridden
				"PORT":     "8080",        // From base
				"DEBUG":    "false",       // From prod
				"SECRET":   "prod-secret", // From prod
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := manager.LoadFiles(tt.files)
			assert.NoError(t, err)
			assert.NotNil(t, file)
			
			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, file.Variables[key].Value)
			}
		})
	}
}

func TestSaveFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create a file to save
	file := NewFile()
	file.Set("APP_NAME", "test-app")
	file.Set("PORT", "3000")
	file.Set("DEBUG", "true")

	// Save the file
	err := manager.SaveFile(".env.test", file)
	assert.NoError(t, err)

	// Verify the file was created
	savedPath := filepath.Join(tmpDir, ".env.test")
	_, err = os.Stat(savedPath)
	assert.NoError(t, err)

	// Load and verify content
	loadedFile, err := manager.LoadFile(".env.test")
	assert.NoError(t, err)
	assert.Equal(t, "test-app", loadedFile.Variables["APP_NAME"].Value)
	assert.Equal(t, "3000", loadedFile.Variables["PORT"].Value)
	assert.Equal(t, "true", loadedFile.Variables["DEBUG"].Value)
}

func TestListFiles(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create test files
	testFiles := []string{
		".env",
		".env.dev",
		".env.prod",
		".env.staging",
		"not-env-file.txt",
		"README.md",
	}

	for _, filename := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("TEST=value"), 0644)
		require.NoError(t, err)
	}

	// List files
	files, err := manager.ListFiles()
	assert.NoError(t, err)
	assert.Equal(t, 4, len(files)) // Only .env files

	// Verify all .env files are found
	expectedFiles := map[string]bool{
		".env":         false,
		".env.dev":     false,
		".env.prod":    false,
		".env.staging": false,
	}

	for _, file := range files {
		expectedFiles[file] = true
	}

	for file, found := range expectedFiles {
		assert.True(t, found, "Expected file %s not found", file)
	}
}

func TestExportToEnvironment(t *testing.T) {
	file := NewFile()
	file.Set("TEST_APP", "myapp")
	file.Set("TEST_PORT", "8080")

	// Export to environment
	manager := NewManager(".")
	err := manager.ExportToEnvironment(file)
	assert.NoError(t, err)

	// Verify environment variables are set
	assert.Equal(t, "myapp", os.Getenv("TEST_APP"))
	assert.Equal(t, "8080", os.Getenv("TEST_PORT"))

	// Clean up
	os.Unsetenv("TEST_APP")
	os.Unsetenv("TEST_PORT")
}

func TestImportFromEnvironment(t *testing.T) {
	// Set test environment variables
	os.Setenv("IMPORT_TEST_1", "value1")
	os.Setenv("IMPORT_TEST_2", "value2")
	os.Setenv("OTHER_VAR", "other-value")
	defer os.Unsetenv("IMPORT_TEST_1")
	defer os.Unsetenv("IMPORT_TEST_2")
	defer os.Unsetenv("OTHER_VAR")

	manager := NewManager(".")
	prefix := "IMPORT_TEST_"

	file := manager.ImportFromEnvironment(prefix)
	assert.NotNil(t, file)
	assert.Equal(t, 2, len(file.Variables))
	assert.Equal(t, "value1", file.Variables["IMPORT_TEST_1"].Value)
	assert.Equal(t, "value2", file.Variables["IMPORT_TEST_2"].Value)
	// OTHER_VAR should not be imported
	_, exists := file.Variables["OTHER_VAR"]
	assert.False(t, exists)
}

func TestDiff(t *testing.T) {
	manager := NewManager(".")

	file1 := NewFile()
	file1.Set("SAME", "same-value")
	file1.Set("MODIFIED", "old-value")
	file1.Set("DELETED", "will-be-deleted")

	file2 := NewFile()
	file2.Set("SAME", "same-value")
	file2.Set("MODIFIED", "new-value")
	file2.Set("ADDED", "newly-added")

	diff := manager.Diff(file1, file2)
	assert.NotNil(t, diff)
	assert.Equal(t, 1, len(diff.Added))
	assert.Equal(t, 1, len(diff.Removed))
	assert.Equal(t, 1, len(diff.Modified))

	assert.Equal(t, "newly-added", diff.Added["ADDED"])
	assert.Equal(t, "will-be-deleted", diff.Removed["DELETED"])
	assert.Equal(t, "old-value", diff.Modified["MODIFIED"].OldValue)
	assert.Equal(t, "new-value", diff.Modified["MODIFIED"].NewValue)
}

func TestDiffIsEmpty(t *testing.T) {
	emptyDiff := &DiffResult{
		Added:    map[string]string{},
		Removed:  map[string]string{},
		Modified: map[string]DiffEntry{},
	}
	assert.True(t, emptyDiff.IsEmpty())

	nonEmptyDiff := &DiffResult{
		Added:    map[string]string{"KEY": "value"},
		Removed:  map[string]string{},
		Modified: map[string]DiffEntry{},
	}
	assert.False(t, nonEmptyDiff.IsEmpty())
}

func TestDiffSummary(t *testing.T) {
	diff := &DiffResult{
		Added:    map[string]string{"NEW": "value"},
		Removed:  map[string]string{"OLD": "value"},
		Modified: map[string]DiffEntry{
			"CHANGED": {OldValue: "old", NewValue: "new"},
		},
	}

	summary := diff.Summary()
	assert.Contains(t, summary, "added")
	assert.Contains(t, summary, "removed")
	assert.Contains(t, summary, "modified")
}

func TestLoadFileNotFound(t *testing.T) {
	manager := NewManager(".")
	_, err := manager.LoadFile("non-existent-file.env")
	assert.Error(t, err)
}

func TestSaveFileInvalidPath(t *testing.T) {
	manager := NewManager("/invalid/path/that/does/not/exist")
	file := NewFile()
	file.Set("KEY", "value")
	
	err := manager.SaveFile(".env", file)
	assert.Error(t, err)
}