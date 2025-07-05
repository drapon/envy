package version

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/version"
	"github.com/stretchr/testify/assert"
)

func TestNewCommand(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("command_structure", func(t *testing.T) {
		cmd := NewCommand()
		
		assert.Equal(t, "version", cmd.Use)
		assert.Equal(t, "Show version information", cmd.Short)
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
		assert.NotNil(t, cmd.RunE)
	})

	t.Run("flags", func(t *testing.T) {
		cmd := NewCommand()

		// Check detailed flag
		detailedFlag := cmd.Flag("detailed")
		assert.NotNil(t, detailedFlag)
		assert.Equal(t, "bool", detailedFlag.Value.Type())
		assert.Equal(t, "false", detailedFlag.DefValue)

		// Check check-update flag
		checkUpdateFlag := cmd.Flag("check-update")
		assert.NotNil(t, checkUpdateFlag)
		assert.Equal(t, "bool", checkUpdateFlag.Value.Type())

		// Check no-color flag
		noColorFlag := cmd.Flag("no-color")
		assert.NotNil(t, noColorFlag)
		assert.Equal(t, "bool", noColorFlag.Value.Type())

		// Check update-prompt flag
		updatePromptFlag := cmd.Flag("update-prompt")
		assert.NotNil(t, updatePromptFlag)
		assert.Equal(t, "bool", updatePromptFlag.Value.Type())
		assert.Equal(t, "true", updatePromptFlag.DefValue)
	})
}

func TestRunVersion(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	// Capture stdout
	oldStdout := os.Stdout

	t.Run("simple_version", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stdout = w

		opts := &Options{
			Detailed:     false,
			CheckUpdate:  false,
			NoColor:      true,
			UpdatePrompt: false,
		}

		err := runVersion(context.Background(), opts)
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Should contain version string
		info := version.GetInfo()
		assert.Contains(t, output, info.String())
	})

	t.Run("detailed_version", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stdout = w

		opts := &Options{
			Detailed:     true,
			CheckUpdate:  false,
			NoColor:      true,
			UpdatePrompt: false,
		}

		err := runVersion(context.Background(), opts)
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Should contain detailed information
		assert.Contains(t, output, "envy version")
		assert.Contains(t, output, "Git commit:")
		assert.Contains(t, output, "Build date:")
		assert.Contains(t, output, "Go version:")
		assert.Contains(t, output, "Compiler:")
		assert.Contains(t, output, "Platform:")
	})

	t.Run("with_color", func(t *testing.T) {
		r, w, _ := os.Pipe()
		os.Stdout = w

		opts := &Options{
			Detailed:     false,
			CheckUpdate:  false,
			NoColor:      false,
			UpdatePrompt: false,
		}

		err := runVersion(context.Background(), opts)
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Should contain color codes
		assert.Contains(t, output, "\033[")
	})

	t.Run("check_update_disabled", func(t *testing.T) {
		_, w, _ := os.Pipe()
		os.Stdout = w

		// Use a very short timeout to make update check fail quickly
		ctx, cancel := context.WithTimeout(context.Background(), 1)
		defer cancel()

		opts := &Options{
			Detailed:     false,
			CheckUpdate:  true,
			NoColor:      true,
			UpdatePrompt: false,
		}

		err := runVersion(ctx, opts)
		assert.NoError(t, err) // Should not return error even if update check fails

		w.Close()
		os.Stdout = oldStdout
	})
}

func TestVersionOptions(t *testing.T) {
	t.Run("default_options", func(t *testing.T) {
		opts := &Options{}
		
		assert.False(t, opts.Detailed)
		assert.False(t, opts.CheckUpdate)
		assert.False(t, opts.NoColor)
		assert.False(t, opts.UpdatePrompt)
	})

	t.Run("all_options_enabled", func(t *testing.T) {
		opts := &Options{
			Detailed:     true,
			CheckUpdate:  true,
			NoColor:      true,
			UpdatePrompt: true,
		}
		
		assert.True(t, opts.Detailed)
		assert.True(t, opts.CheckUpdate)
		assert.True(t, opts.NoColor)
		assert.True(t, opts.UpdatePrompt)
	})
}

func TestVersionOutputFormat(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	testCases := []struct {
		name        string
		opts        *Options
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "simple_format",
			opts: &Options{
				NoColor:      true,
				UpdatePrompt: false,
			},
			checkOutput: func(t *testing.T, output string) {
				// Should be a single line
				trimmed := strings.TrimSpace(output)
				if trimmed != "" {
					lines := strings.Split(trimmed, "\n")
					assert.Equal(t, 1, len(lines))
				}
				// Should contain version
				assert.Contains(t, output, "envy")
			},
		},
		{
			name: "detailed_format",
			opts: &Options{
				Detailed:     true,
				NoColor:      true,
				UpdatePrompt: false,
			},
			checkOutput: func(t *testing.T, output string) {
				// Should have multiple lines
				lines := strings.Split(strings.TrimSpace(output), "\n")
				assert.Greater(t, len(lines), 5)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := runVersion(context.Background(), tc.opts)
			assert.NoError(t, err)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			tc.checkOutput(t, output)
		})
	}
}

func TestVersionCommandIntegration(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("command_execution", func(t *testing.T) {
		cmd := NewCommand()
		
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Execute command
		cmd.SetArgs([]string{"--no-color"})
		err := cmd.Execute()
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Verify output contains version info
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "envy")
	})

	t.Run("command_with_detailed_flag", func(t *testing.T) {
		cmd := NewCommand()
		
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Execute command with detailed flag
		cmd.SetArgs([]string{"--detailed", "--no-color"})
		err := cmd.Execute()
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Verify detailed output
		assert.Contains(t, output, "Git commit:")
		assert.Contains(t, output, "Build date:")
	})
}

// BenchmarkRunVersion benchmarks the version command
func BenchmarkRunVersion(b *testing.B) {
	// Disable output
	oldStdout := os.Stdout
	os.Stdout = nil
	defer func() {
		os.Stdout = oldStdout
	}()

	opts := &Options{
		Detailed:     false,
		CheckUpdate:  false,
		NoColor:      true,
		UpdatePrompt: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = runVersion(context.Background(), opts)
	}
}