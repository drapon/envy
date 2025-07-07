package root

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func()
		testFunc  func(t *testing.T)
	}{
		{
			name: "GetRootCmd returns non-nil command",
			testFunc: func(t *testing.T) {
				cmd := GetRootCmd()
				assert.NotNil(t, cmd)
				assert.Equal(t, "envy", cmd.Use)
				assert.Contains(t, cmd.Short, "environment variable")
			},
		},
		{
			name: "IsDebug returns false by default",
			testFunc: func(t *testing.T) {
				assert.False(t, IsDebug())
			},
		},
		{
			name: "IsVerbose returns false by default",
			testFunc: func(t *testing.T) {
				assert.False(t, IsVerbose())
			},
		},
		{
			name: "IsNoColor returns false by default",
			testFunc: func(t *testing.T) {
				assert.False(t, IsNoColor())
			},
		},
		{
			name: "IsNoCache returns false by default",
			testFunc: func(t *testing.T) {
				assert.False(t, IsNoCache())
			},
		},
		{
			name: "IsClearCache returns false by default",
			testFunc: func(t *testing.T) {
				assert.False(t, IsClearCache())
			},
		},
		{
			name: "AddCommand adds command successfully",
			testFunc: func(t *testing.T) {
				testCmd := &cobra.Command{
					Use:   "test",
					Short: "Test command",
				}
				AddCommand(testCmd)
				
				// Verify command was added
				found := false
				for _, cmd := range GetRootCmd().Commands() {
					if cmd.Use == "test" {
						found = true
						break
					}
				}
				assert.True(t, found, "Test command should be added to root command")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}
			tt.testFunc(t)
		})
	}
}

func TestExecute(t *testing.T) {
	// Skip this test as it requires full initialization
	t.Skip("Execute requires full application initialization")
}