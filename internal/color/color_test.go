package color

import (
	"testing"
)

func TestColorFormats(t *testing.T) {
	// Test that color functions don't panic
	tests := []struct {
		name     string
		function func(string) string
		input    string
	}{
		{"Success", FormatSuccess, "This is a success message"},
		{"Error", FormatError, "This is an error message"},
		{"Warning", FormatWarning, "This is a warning message"},
		{"Info", FormatInfo, "This is an info message"},
		{"Bold", FormatBold, "This is a bold message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.input)
			if result == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
		})
	}
}

func TestPrintFunctions(t *testing.T) {
	// Test that print functions don't panic
	PrintSuccessf("Test success message")
	PrintErrorf("Test error message")
	PrintWarningf("Test warning message")
	PrintInfof("Test info message")
	PrintBoldf("Test bold message")

	// Test with format strings
	PrintSuccessf("Test success with %s", "formatting")
	PrintErrorf("Test error with number %d", 42)
}

func TestColorToggle(t *testing.T) {
	// Test color enable/disable
	DisableColors()
	result := FormatSuccess("test")
	if result != "test" {
		t.Error("DisableColors() didn't remove color formatting")
	}

	EnableColors()
	result = FormatSuccess("test")
	if result == "test" {
		t.Error("EnableColors() didn't add color formatting")
	}
}
