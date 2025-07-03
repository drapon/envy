package version

import (
	"strings"
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()

	if info == nil {
		t.Fatal("GetInfo() returned nil")
	}

	// Version should not be empty
	if info.Version == "" {
		t.Error("Version should not be empty")
	}

	// Go version should start with "go"
	if !strings.HasPrefix(info.GoVersion, "go") {
		t.Errorf("GoVersion should start with 'go', got: %s", info.GoVersion)
	}

	// Platform should contain "/"
	if !strings.Contains(info.Platform, "/") {
		t.Errorf("Platform should contain '/', got: %s", info.Platform)
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"v1 newer major", "2.0.0", "1.0.0", 1},
		{"v1 older major", "1.0.0", "2.0.0", -1},
		{"v1 newer minor", "1.2.0", "1.1.0", 1},
		{"v1 older minor", "1.1.0", "1.2.0", -1},
		{"v1 newer patch", "1.0.2", "1.0.1", 1},
		{"v1 older patch", "1.0.1", "1.0.2", -1},
		{"with v prefix", "v1.2.3", "v1.2.3", 0},
		{"mixed v prefix", "v1.2.3", "1.2.3", 0},
		{"dev version v1", "dev", "1.0.0", -1},
		{"dev version v2", "1.0.0", "dev", 1},
		{"both dev", "dev", "dev", 0},
		{"pre-release ignored", "1.0.0-beta", "1.0.0-alpha", 0},
		{"different lengths", "2.0", "1.0.0", 1},
		{"empty parts", "1", "1.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Compare(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("Compare(%s, %s) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{"v1 is newer", "2.0.0", "1.0.0", true},
		{"v1 is older", "1.0.0", "2.0.0", false},
		{"versions equal", "1.0.0", "1.0.0", false},
		{"dev version", "1.0.0", "dev", true},
		{"with v prefix", "v2.0.0", "v1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNewer(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("IsNewer(%s, %s) = %v, want %v", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestString(t *testing.T) {
	info := GetInfo()
	str := info.String()

	if !strings.Contains(str, "envy version") {
		t.Errorf("String() should contain 'envy version', got: %s", str)
	}

	if !strings.Contains(str, info.Version) {
		t.Errorf("String() should contain version %s, got: %s", info.Version, str)
	}
}

func TestDetailedString(t *testing.T) {
	info := GetInfo()
	str := info.DetailedString()

	// Check that all fields are present
	expectedFields := []string{
		"envy version",
		"Git commit:",
		"Build date:",
		"Go version:",
		"Compiler:",
		"Platform:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(str, field) {
			t.Errorf("DetailedString() should contain '%s', got: %s", field, str)
		}
	}
}
