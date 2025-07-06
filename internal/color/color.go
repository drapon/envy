// Package color provides colored output formatting utilities.
package color

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/viper"
)

var (
	// Color functions for different message types.
	Success = color.New(color.FgGreen).SprintFunc()
	Error   = color.New(color.FgRed).SprintFunc()
	Warning = color.New(color.FgYellow).SprintFunc()
	Info    = color.New(color.FgCyan).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()

	// Formatted print functions.
	SuccessF = color.New(color.FgGreen).SprintfFunc()
	ErrorF   = color.New(color.FgRed).SprintfFunc()
	WarningF = color.New(color.FgYellow).SprintfFunc()
	InfoF    = color.New(color.FgCyan).SprintfFunc()
	BoldF    = color.New(color.Bold).SprintfFunc()
)

// Initialize checks environment for color settings.
func Initialize() {
	// Check if colors should be disabled
	// Check environment variable first as viper might not be initialized
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
		return
	}
	
	// Check viper config if available
	if viper.IsSet("no_color") && viper.GetBool("no_color") {
		color.NoColor = true
	}
}

// PrintSuccessf prints a success message in green.
func PrintSuccessf(format string, args ...interface{}) {
	fmt.Println(SuccessF(format, args...))
}

// PrintErrorf prints an error message in red.
func PrintErrorf(format string, args ...interface{}) {
	fmt.Println(ErrorF(format, args...))
}

// PrintWarningf prints a warning message in yellow.
func PrintWarningf(format string, args ...interface{}) {
	fmt.Println(WarningF(format, args...))
}

// PrintInfof prints an info message in cyan.
func PrintInfof(format string, args ...interface{}) {
	fmt.Println(InfoF(format, args...))
}

// PrintBoldf prints a bold message.
func PrintBoldf(format string, args ...interface{}) {
	fmt.Println(BoldF(format, args...))
}

// FormatSuccess returns a success-formatted string.
func FormatSuccess(text string) string {
	return Success(text)
}

// FormatError returns an error-formatted string.
func FormatError(text string) string {
	return Error(text)
}

// FormatWarning returns a warning-formatted string.
func FormatWarning(text string) string {
	return Warning(text)
}

// FormatInfo returns an info-formatted string.
func FormatInfo(text string) string {
	return Info(text)
}

// FormatBold returns a bold-formatted string.
func FormatBold(text string) string {
	return Bold(text)
}

// DisableColors disables all color output.
func DisableColors() {
	color.NoColor = true
}

// EnableColors enables color output.
func EnableColors() {
	color.NoColor = false
}
