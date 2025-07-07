package color

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorFunctions(t *testing.T) {
	// Initialize colors
	Initialize()

	tests := []struct {
		name     string
		testFunc func()
	}{
		{
			name: "DisableColors",
			testFunc: func() {
				DisableColors()
				// After disabling, format functions should return plain text
				assert.Equal(t, "test", FormatSuccess("test"))
				assert.Equal(t, "test", FormatError("test"))
				assert.Equal(t, "test", FormatWarning("test"))
				assert.Equal(t, "test", FormatInfo("test"))
				assert.Equal(t, "test", FormatBold("test"))
			},
		},
		{
			name: "EnableColors",
			testFunc: func() {
				EnableColors()
				// Format functions work without panic
				_ = FormatSuccess("test")
				_ = FormatError("test")
				_ = FormatWarning("test")
				_ = FormatInfo("test")
				_ = FormatBold("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc()
		})
	}
}

func TestInitialize(t *testing.T) {
	// Test initialization
	Initialize()
	// No assertion needed, just ensure it doesn't panic
}