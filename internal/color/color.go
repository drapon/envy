package color

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/viper"
)

var (
	// Color functions for different message types
	Success = color.New(color.FgGreen).SprintFunc()
	Error   = color.New(color.FgRed).SprintFunc()
	Warning = color.New(color.FgYellow).SprintFunc()
	Info    = color.New(color.FgCyan).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	
	// Formatted print functions
	SuccessF = color.New(color.FgGreen).SprintfFunc()
	ErrorF   = color.New(color.FgRed).SprintfFunc()
	WarningF = color.New(color.FgYellow).SprintfFunc()
	InfoF    = color.New(color.FgCyan).SprintfFunc()
	BoldF    = color.New(color.Bold).SprintfFunc()
)

func init() {
	// Check if colors should be disabled
	if viper.GetBool("no_color") || os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}

// PrintSuccess prints a success message in green
func PrintSuccess(format string, args ...interface{}) {
	fmt.Println(SuccessF(format, args...))
}

// PrintError prints an error message in red
func PrintError(format string, args ...interface{}) {
	fmt.Println(ErrorF(format, args...))
}

// PrintWarning prints a warning message in yellow
func PrintWarning(format string, args ...interface{}) {
	fmt.Println(WarningF(format, args...))
}

// PrintInfo prints an info message in cyan
func PrintInfo(format string, args ...interface{}) {
	fmt.Println(InfoF(format, args...))
}

// PrintBold prints a bold message
func PrintBold(format string, args ...interface{}) {
	fmt.Println(BoldF(format, args...))
}

// FormatSuccess returns a success-formatted string
func FormatSuccess(text string) string {
	return Success(text)
}

// FormatError returns an error-formatted string
func FormatError(text string) string {
	return Error(text)
}

// FormatWarning returns a warning-formatted string
func FormatWarning(text string) string {
	return Warning(text)
}

// FormatInfo returns an info-formatted string
func FormatInfo(text string) string {
	return Info(text)
}

// FormatBold returns a bold-formatted string
func FormatBold(text string) string {
	return Bold(text)
}

// DisableColors disables all color output
func DisableColors() {
	color.NoColor = true
}

// EnableColors enables color output
func EnableColors() {
	color.NoColor = false
}