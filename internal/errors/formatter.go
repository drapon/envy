package errors

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Formatter provides error formatting functionality
type Formatter struct {
	useColor bool
	verbose  bool
}

// NewFormatter creates a new error formatter
func NewFormatter(useColor, verbose bool) *Formatter {
	return &Formatter{
		useColor: useColor,
		verbose:  verbose,
	}
}

// Format formats an error for display
func (f *Formatter) Format(err error) string {
	if err == nil {
		return ""
	}

	var envyErr *EnvyError
	if !stderrors.As(err, &envyErr) {
		// Not an EnvyError, return as-is
		return err.Error()
	}

	var builder strings.Builder

	// Error header with code
	if f.useColor {
		builder.WriteString(color.RedString("エラー [%s]: ", envyErr.Code))
	} else {
		builder.WriteString(fmt.Sprintf("エラー [%s]: ", envyErr.Code))
	}

	// User-friendly message
	builder.WriteString(envyErr.UserMessage())
	builder.WriteString("\n")

	// Details if available
	if len(envyErr.Details) > 0 && f.verbose {
		builder.WriteString("\n詳細情報:\n")
		for key, value := range envyErr.Details {
			builder.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	// Original error in verbose mode
	if f.verbose && envyErr.Cause != nil {
		builder.WriteString("\n原因:\n")
		builder.WriteString(fmt.Sprintf("  %v\n", envyErr.Cause))
	}

	// Suggestions based on error type
	suggestion := f.getSuggestion(envyErr)
	if suggestion != "" {
		builder.WriteString("\n")
		if f.useColor {
			builder.WriteString(color.YellowString("対処法: "))
		} else {
			builder.WriteString("対処法: ")
		}
		builder.WriteString(suggestion)
		builder.WriteString("\n")
	}

	// Retry hint if applicable
	if envyErr.Retriable {
		builder.WriteString("\n")
		if f.useColor {
			builder.WriteString(color.CyanString("このエラーは一時的なものです。再試行してください。"))
		} else {
			builder.WriteString("このエラーは一時的なものです。再試行してください。")
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// FormatShort formats an error in a compact way
func (f *Formatter) FormatShort(err error) string {
	if err == nil {
		return ""
	}

	var envyErr *EnvyError
	if !stderrors.As(err, &envyErr) {
		return err.Error()
	}

	if f.useColor {
		return color.RedString("[%s] %s", envyErr.Code, envyErr.UserMessage())
	}
	return fmt.Sprintf("[%s] %s", envyErr.Code, envyErr.UserMessage())
}

// FormatMultiple formats multiple errors
func (f *Formatter) FormatMultiple(errors []error) string {
	if len(errors) == 0 {
		return ""
	}

	var builder strings.Builder

	if f.useColor {
		builder.WriteString(color.RedString("複数のエラーが発生しました:\n"))
	} else {
		builder.WriteString("複数のエラーが発生しました:\n")
	}

	for i, err := range errors {
		builder.WriteString(fmt.Sprintf("\n%d. ", i+1))

		var envyErr *EnvyError
		if stderrors.As(err, &envyErr) {
			builder.WriteString(f.FormatShort(err))
		} else {
			builder.WriteString(err.Error())
		}
	}

	return builder.String()
}

// getSuggestion returns a suggestion based on the error type
func (f *Formatter) getSuggestion(err *EnvyError) string {
	switch err.Code {
	case ErrConfigNotFound:
		return "'envy init' コマンドを実行して初期設定を行ってください。"

	case ErrConfigInvalid:
		return ".envyrcファイルの構文を確認し、YAMLフォーマットが正しいことを確認してください。"

	case ErrAWSAuth:
		return `以下を確認してください:
  1. AWS認証情報が正しく設定されているか
  2. ~/.aws/credentials ファイルが存在するか
  3. AWS_ACCESS_KEY_ID と AWS_SECRET_ACCESS_KEY 環境変数が設定されているか
  4. IAMロールを使用している場合は、適切な権限があるか`

	case ErrAWSAccessDenied:
		return `IAMポリシーに以下の権限があることを確認してください:
  - ssm:GetParameter, ssm:PutParameter (Parameter Store使用時)
  - secretsmanager:GetSecretValue, secretsmanager:CreateSecret (Secrets Manager使用時)
  - kms:Decrypt (暗号化されたパラメータ使用時)`

	case ErrFilePermission:
		if file, ok := err.Details["file"].(string); ok {
			return fmt.Sprintf("'chmod 600 %s' コマンドでファイルの権限を修正してください。", file)
		}
		return "ファイルの権限を確認してください。"

	case ErrInvalidEnvironment:
		return "'envy list environments' コマンドで利用可能な環境を確認してください。"

	case ErrParameterNotFound, ErrSecretNotFound:
		return "'envy list' コマンドで利用可能なパラメータを確認してください。"

	case ErrNetworkTimeout, ErrNetworkUnavailable:
		return "インターネット接続を確認し、プロキシ設定が必要な場合は環境変数 HTTP_PROXY, HTTPS_PROXY を設定してください。"

	case ErrAWSRateLimit:
		return "しばらく待ってから再試行するか、AWS APIの呼び出し頻度を下げてください。"

	default:
		return ""
	}
}

// PrintError prints an error to stderr with formatting.
func PrintError(err error) {
	formatter := NewFormatter(true, false)
	_, _ = fmt.Fprintln(color.Error, formatter.Format(err))
}

// PrintErrorVerbose prints an error with verbose information.
func PrintErrorVerbose(err error) {
	formatter := NewFormatter(true, true)
	_, _ = fmt.Fprintln(color.Error, formatter.Format(err))
}

// PrintWarning prints a warning message.
func PrintWarning(message string) {
	_, _ = fmt.Fprintln(color.Error, color.YellowString("警告: %s", message))
}

// PrintSuccess prints a success message.
func PrintSuccessf(format string, args ...interface{}) {
	fmt.Println(color.GreenString("✓ "+format, args...))
}

// PrintInfo prints an info message.
func PrintInfof(format string, args ...interface{}) {
	fmt.Println(color.CyanString("ℹ "+format, args...))
}

// ErrorContext provides additional context for errors
type ErrorContext struct {
	Operation   string
	Environment string
	Region      string
	Profile     string
	File        string
}

// FormatWithContext formats an error with additional context
func FormatWithContext(err error, ctx ErrorContext) string {
	var builder strings.Builder

	// Main error
	formatter := NewFormatter(true, false)
	builder.WriteString(formatter.Format(err))

	// Context information
	builder.WriteString("\nコンテキスト:\n")

	if ctx.Operation != "" {
		builder.WriteString(fmt.Sprintf("  操作: %s\n", ctx.Operation))
	}
	if ctx.Environment != "" {
		builder.WriteString(fmt.Sprintf("  環境: %s\n", ctx.Environment))
	}
	if ctx.Region != "" {
		builder.WriteString(fmt.Sprintf("  リージョン: %s\n", ctx.Region))
	}
	if ctx.Profile != "" {
		builder.WriteString(fmt.Sprintf("  プロファイル: %s\n", ctx.Profile))
	}
	if ctx.File != "" {
		builder.WriteString(fmt.Sprintf("  ファイル: %s\n", ctx.File))
	}

	return builder.String()
}
