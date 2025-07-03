package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager manages environment files and operations
type Manager struct {
	basePath string
}

// NewManager creates a new environment manager
func NewManager(basePath string) *Manager {
	if basePath == "" {
		basePath = "."
	}
	return &Manager{
		basePath: basePath,
	}
}

// LoadFile loads an environment file
func (m *Manager) LoadFile(filename string) (*File, error) {
	path := filepath.Join(m.basePath, filename)
	return ParseFile(path)
}

// LoadFiles loads multiple environment files and merges them
func (m *Manager) LoadFiles(filenames []string) (*File, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files specified")
	}

	// Load first file
	result, err := m.LoadFile(filenames[0])
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", filenames[0], err)
	}

	// Merge additional files
	for _, filename := range filenames[1:] {
		file, err := m.LoadFile(filename)
		if err != nil {
			// Skip if file doesn't exist
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to load %s: %w", filename, err)
		}
		result.Merge(file)
	}

	return result, nil
}

// SaveFile saves an environment file
func (m *Manager) SaveFile(filename string, file *File) error {
	path := filepath.Join(m.basePath, filename)
	
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return file.WriteFile(path)
}

// ListFiles lists all .env files in the base path
func (m *Manager) ListFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(m.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's an .env file
		name := info.Name()
		if strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".env") {
			relPath, err := filepath.Rel(m.basePath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// ExportToEnvironment exports variables to the current process environment
func (m *Manager) ExportToEnvironment(file *File) error {
	for key, variable := range file.Variables {
		if err := os.Setenv(key, variable.Value); err != nil {
			return fmt.Errorf("failed to set %s: %w", key, err)
		}
	}
	return nil
}

// ImportFromEnvironment imports variables from the current process environment
func (m *Manager) ImportFromEnvironment(prefix string) *File {
	file := NewFile()

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Filter by prefix if specified
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}

		file.Set(key, value)
	}

	return file
}

// Diff compares two environment files
func (m *Manager) Diff(file1, file2 *File) *DiffResult {
	result := &DiffResult{
		Added:    make(map[string]string),
		Removed:  make(map[string]string),
		Modified: make(map[string]DiffEntry),
	}

	// Find added and modified variables
	for key, var2 := range file2.Variables {
		if var1, exists := file1.Variables[key]; exists {
			// Modified
			if var1.Value != var2.Value {
				result.Modified[key] = DiffEntry{
					OldValue: var1.Value,
					NewValue: var2.Value,
				}
			}
		} else {
			// Added
			result.Added[key] = var2.Value
		}
	}

	// Find removed variables
	for key, var1 := range file1.Variables {
		if _, exists := file2.Variables[key]; !exists {
			result.Removed[key] = var1.Value
		}
	}

	return result
}

// DiffResult represents the difference between two environment files
type DiffResult struct {
	Added    map[string]string
	Removed  map[string]string
	Modified map[string]DiffEntry
}

// DiffEntry represents a modified variable
type DiffEntry struct {
	OldValue string
	NewValue string
}

// IsEmpty returns true if there are no differences
func (d *DiffResult) IsEmpty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0
}

// Summary returns a summary of the differences
func (d *DiffResult) Summary() string {
	parts := []string{}
	
	if len(d.Added) > 0 {
		parts = append(parts, fmt.Sprintf("%d added", len(d.Added)))
	}
	if len(d.Removed) > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", len(d.Removed)))
	}
	if len(d.Modified) > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", len(d.Modified)))
	}

	if len(parts) == 0 {
		return "No changes"
	}

	return strings.Join(parts, ", ")
}