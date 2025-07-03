package errors

import (
	"errors"
	"strings"

	"github.com/aws/smithy-go"
)

// IsConfigError checks if the error is a configuration error
func IsConfigError(err error) bool {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		switch envyErr.Code {
		case ErrConfigNotFound, ErrConfigInvalid, ErrConfigParse, ErrConfigPermission:
			return true
		}
	}
	return false
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		switch envyErr.Code {
		case ErrValidationFailed, ErrInvalidArgument, ErrInvalidEnvironment,
			ErrInvalidKeyFormat, ErrRequiredField:
			return true
		}
	}
	return false
}

// IsAWSError checks if the error is an AWS error
func IsAWSError(err error) bool {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		switch envyErr.Code {
		case ErrAWSAuth, ErrAWSConnection, ErrAWSRateLimit, ErrAWSAccessDenied,
			ErrParameterNotFound, ErrSecretNotFound, ErrParameterExists,
			ErrSecretExists, ErrAWSTimeout:
			return true
		}
	}
	// Check for AWS SDK errors
	var apiErr smithy.APIError
	return errors.As(err, &apiErr)
}

// IsFileError checks if the error is a file error
func IsFileError(err error) bool {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		switch envyErr.Code {
		case ErrFileNotFound, ErrFilePermission, ErrFileRead, ErrFileWrite, ErrFileInvalid:
			return true
		}
	}
	return false
}

// IsNetworkError checks if the error is a network error
func IsNetworkError(err error) bool {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		switch envyErr.Code {
		case ErrNetworkTimeout, ErrNetworkUnavailable, ErrDNSResolution:
			return true
		}
	}
	// Check for common network error patterns
	errStr := err.Error()
	networkPatterns := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"timeout",
		"network is unreachable",
		"connection timed out",
	}
	for _, pattern := range networkPatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}

// IsRetriable checks if the error is retriable
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}

	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		if envyErr.Retriable {
			return true
		}
		// Certain error codes are always retriable
		switch envyErr.Code {
		case ErrAWSRateLimit, ErrAWSTimeout, ErrNetworkTimeout:
			return true
		}
	}

	// Check for AWS throttling errors
	if IsAWSError(err) {
		errStr := err.Error()
		retriablePatterns := []string{
			"Throttling",
			"Rate exceeded",
			"TooManyRequestsException",
			"RequestLimitExceeded",
			"ServiceUnavailable",
			"RequestTimeout",
		}
		for _, pattern := range retriablePatterns {
			if strings.Contains(errStr, pattern) {
				return true
			}
		}
	}

	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		return envyErr.Code
	}
	return ErrUnknown
}

// GetErrorDetails extracts error details
func GetErrorDetails(err error) map[string]interface{} {
	var envyErr *EnvyError
	if errors.As(err, &envyErr) {
		return envyErr.Details
	}
	return nil
}

// Wrap wraps an existing error with EnvyError
func Wrap(err error, code ErrorCode, message string) *EnvyError {
	if err == nil {
		return nil
	}
	return New(code, message).WithCause(err)
}

// WrapAWSError converts AWS SDK errors to EnvyError
func WrapAWSError(err error, operation string, resource string) *EnvyError {
	if err == nil {
		return nil
	}

	// Map specific AWS errors
	errStr := err.Error()

	// Authentication errors
	if strings.Contains(errStr, "AccessDenied") ||
		strings.Contains(errStr, "UnauthorizedOperation") ||
		strings.Contains(errStr, "is not authorized to perform") {
		return New(ErrAWSAccessDenied, "AWS access denied").
			WithCause(err).
			WithDetails("operation", operation).
			WithDetails("resource", resource)
	}

	// Not found errors
	if strings.Contains(errStr, "ParameterNotFound") ||
		strings.Contains(errStr, "Parameter not found") {
		return New(ErrParameterNotFound, "Parameter not found").
			WithCause(err).
			WithDetails("operation", operation).
			WithDetails("parameter", resource)
	}

	if strings.Contains(errStr, "ResourceNotFoundException") ||
		strings.Contains(errStr, "Secret not found") {
		return New(ErrSecretNotFound, "Secret not found").
			WithCause(err).
			WithDetails("operation", operation).
			WithDetails("secret", resource)
	}

	// Already exists errors
	if strings.Contains(errStr, "ParameterAlreadyExists") {
		return New(ErrParameterExists, "Parameter already exists").
			WithCause(err).
			WithDetails("operation", operation).
			WithDetails("parameter", resource)
	}

	if strings.Contains(errStr, "ResourceExistsException") {
		return New(ErrSecretExists, "Secret already exists").
			WithCause(err).
			WithDetails("operation", operation).
			WithDetails("secret", resource)
	}

	// Rate limiting
	if strings.Contains(errStr, "Throttling") ||
		strings.Contains(errStr, "Rate exceeded") ||
		strings.Contains(errStr, "TooManyRequestsException") {
		return New(ErrAWSRateLimit, "AWS API rate limit exceeded").
			WithCause(err).
			WithRetriable(true).
			WithDetails("operation", operation)
	}

	// Timeout
	if strings.Contains(errStr, "RequestTimeout") ||
		strings.Contains(errStr, "timeout") {
		return New(ErrAWSTimeout, "AWS request timeout").
			WithCause(err).
			WithRetriable(true).
			WithDetails("operation", operation)
	}

	// Generic AWS error
	return New(ErrAWSConnection, "AWS operation failed").
		WithCause(err).
		WithDetails("operation", operation).
		WithDetails("resource", resource)
}

// WrapFileError converts file operation errors to EnvyError
func WrapFileError(err error, filepath string) *EnvyError {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	if strings.Contains(errStr, "no such file") ||
		strings.Contains(errStr, "cannot find the file") {
		return New(ErrFileNotFound, "File not found").
			WithCause(err).
			WithDetails("file", filepath)
	}

	if strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "access is denied") {
		return New(ErrFilePermission, "File permission denied").
			WithCause(err).
			WithDetails("file", filepath)
	}

	return New(ErrFileInvalid, "File operation failed").
		WithCause(err).
		WithDetails("file", filepath)
}

// WrapNetworkError converts network errors to EnvyError
func WrapNetworkError(err error) *EnvyError {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "timeout") {
		return New(ErrNetworkTimeout, "Network timeout").
			WithCause(err).
			WithRetriable(true)
	}

	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "cannot resolve") {
		return New(ErrDNSResolution, "DNS resolution failed").
			WithCause(err)
	}

	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "network is unreachable") {
		return New(ErrNetworkUnavailable, "Network unavailable").
			WithCause(err).
			WithRetriable(true)
	}

	return New(ErrNetworkUnavailable, "Network error").
		WithCause(err).
		WithRetriable(true)
}

// AggregateErrors combines multiple errors into one
type ErrorAggregator struct {
	errors []error
}

// NewAggregator creates a new error aggregator
func NewAggregator() *ErrorAggregator {
	return &ErrorAggregator{
		errors: make([]error, 0),
	}
}

// Add adds an error to the aggregator
func (a *ErrorAggregator) Add(err error) {
	if err != nil {
		a.errors = append(a.errors, err)
	}
}

// AddWithContext adds an error with context
func (a *ErrorAggregator) AddWithContext(err error, context string) {
	if err != nil {
		contextErr := Wrap(err, GetErrorCode(err), context)
		a.errors = append(a.errors, contextErr)
	}
}

// HasErrors checks if there are any errors
func (a *ErrorAggregator) HasErrors() bool {
	return len(a.errors) > 0
}

// Error returns the aggregated error
func (a *ErrorAggregator) Error() error {
	if !a.HasErrors() {
		return nil
	}

	if len(a.errors) == 1 {
		return a.errors[0]
	}

	// Create a combined error message
	var messages []string
	for _, err := range a.errors {
		messages = append(messages, err.Error())
	}

	return New(ErrInternal, "Multiple errors occurred").
		WithDetails("errors", messages).
		WithDetails("count", len(a.errors))
}

// Errors returns all collected errors
func (a *ErrorAggregator) Errors() []error {
	return a.errors
}
