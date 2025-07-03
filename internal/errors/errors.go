package errors

import (
	"fmt"
	"time"
)

// ErrorCode represents the type of error
type ErrorCode string

const (
	// Configuration related errors
	ErrConfigNotFound   ErrorCode = "CONFIG_NOT_FOUND"
	ErrConfigInvalid    ErrorCode = "CONFIG_INVALID"
	ErrConfigParse      ErrorCode = "CONFIG_PARSE"
	ErrConfigPermission ErrorCode = "CONFIG_PERMISSION"

	// Validation related errors
	ErrValidationFailed   ErrorCode = "VALIDATION_FAILED"
	ErrInvalidArgument    ErrorCode = "INVALID_ARGUMENT"
	ErrInvalidEnvironment ErrorCode = "INVALID_ENVIRONMENT"
	ErrInvalidKeyFormat   ErrorCode = "INVALID_KEY_FORMAT"
	ErrRequiredField      ErrorCode = "REQUIRED_FIELD"

	// AWS related errors
	ErrAWSAuth           ErrorCode = "AWS_AUTH_FAILED"
	ErrAWSConnection     ErrorCode = "AWS_CONNECTION_FAILED"
	ErrAWSRateLimit      ErrorCode = "AWS_RATE_LIMIT"
	ErrAWSAccessDenied   ErrorCode = "AWS_ACCESS_DENIED"
	ErrParameterNotFound ErrorCode = "PARAMETER_NOT_FOUND"
	ErrSecretNotFound    ErrorCode = "SECRET_NOT_FOUND"
	ErrParameterExists   ErrorCode = "PARAMETER_EXISTS"
	ErrSecretExists      ErrorCode = "SECRET_EXISTS"
	ErrAWSTimeout        ErrorCode = "AWS_TIMEOUT"

	// File related errors
	ErrFileNotFound   ErrorCode = "FILE_NOT_FOUND"
	ErrFilePermission ErrorCode = "FILE_PERMISSION"
	ErrFileRead       ErrorCode = "FILE_READ"
	ErrFileWrite      ErrorCode = "FILE_WRITE"
	ErrFileInvalid    ErrorCode = "FILE_INVALID"

	// Network related errors
	ErrNetworkTimeout     ErrorCode = "NETWORK_TIMEOUT"
	ErrNetworkUnavailable ErrorCode = "NETWORK_UNAVAILABLE"
	ErrDNSResolution      ErrorCode = "DNS_RESOLUTION"

	// System errors
	ErrInternal     ErrorCode = "INTERNAL_ERROR"
	ErrUnknown      ErrorCode = "UNKNOWN_ERROR"
	ErrNotSupported ErrorCode = "NOT_SUPPORTED"
	ErrTimeout      ErrorCode = "TIMEOUT"
	ErrInvalidInput ErrorCode = "INVALID_INPUT"
)

// EnvyError は envy のカスタムエラー型
type EnvyError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Cause     error                  `json:"-"`
	Timestamp time.Time              `json:"timestamp"`
	Retriable bool                   `json:"retriable"`
}

// New は新しいEnvyErrorを作成
func New(code ErrorCode, message string) *EnvyError {
	return &EnvyError{
		Code:      code,
		Message:   message,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Retriable: false,
	}
}

// Error implements error interface
func (e *EnvyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// WithCause は原因となるエラーを設定
func (e *EnvyError) WithCause(cause error) *EnvyError {
	e.Cause = cause
	return e
}

// WithDetails は詳細情報を追加
func (e *EnvyError) WithDetails(key string, value interface{}) *EnvyError {
	e.Details[key] = value
	return e
}

// WithRetriable はリトライ可能フラグを設定
func (e *EnvyError) WithRetriable(retriable bool) *EnvyError {
	e.Retriable = retriable
	return e
}

// Is implements errors.Is
func (e *EnvyError) Is(target error) bool {
	t, ok := target.(*EnvyError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Unwrap implements errors.Unwrap
func (e *EnvyError) Unwrap() error {
	return e.Cause
}

// UserMessage returns user-friendly error messages
func (e *EnvyError) UserMessage() string {
	switch e.Code {
	// Configuration related
	case ErrConfigNotFound:
		return "Configuration file not found. Please run 'envy init' to initialize settings."
	case ErrConfigInvalid:
		return "Configuration file format is invalid. Please check your .envyrc file."
	case ErrConfigParse:
		return "Failed to parse configuration file. Please check YAML format."
	case ErrConfigPermission:
		return "Cannot access configuration file. Please check file permissions."

	// Validation related
	case ErrValidationFailed:
		return "Input validation failed."
	case ErrInvalidArgument:
		return "Invalid argument specified."
	case ErrInvalidEnvironment:
		if env, ok := e.Details["environment"].(string); ok {
			return fmt.Sprintf("Environment '%s' does not exist. Please specify an environment defined in .envyrc file.", env)
		}
		return "Invalid environment specified."
	case ErrInvalidKeyFormat:
		return "Invalid environment variable key format. Only alphanumeric characters and underscores are allowed."
	case ErrRequiredField:
		if field, ok := e.Details["field"].(string); ok {
			return fmt.Sprintf("Required field '%s' is missing.", field)
		}
		return "Required field is missing."

	// AWS related
	case ErrAWSAuth:
		return "AWS authentication failed. Please check credentials and IAM permissions."
	case ErrAWSConnection:
		return "Failed to connect to AWS. Please check network connection."
	case ErrAWSRateLimit:
		return "AWS API rate limit reached. Please wait and try again."
	case ErrAWSAccessDenied:
		return "Access to AWS resource denied. Please check IAM permissions."
	case ErrParameterNotFound:
		if param, ok := e.Details["parameter"].(string); ok {
			return fmt.Sprintf("Parameter '%s' not found.", param)
		}
		return "Specified parameter not found."
	case ErrSecretNotFound:
		if secret, ok := e.Details["secret"].(string); ok {
			return fmt.Sprintf("Secret '%s' not found.", secret)
		}
		return "Specified secret not found."
	case ErrParameterExists:
		return "Parameter already exists. Use --force option to overwrite."
	case ErrSecretExists:
		return "Secret already exists. Use --force option to overwrite."
	case ErrAWSTimeout:
		return "AWS API timeout occurred. Please try again."

	// File related
	case ErrFileNotFound:
		if file, ok := e.Details["file"].(string); ok {
			return fmt.Sprintf("File '%s' not found.", file)
		}
		return "Specified file not found."
	case ErrFilePermission:
		return "No permission to access file."
	case ErrFileRead:
		return "Failed to read file."
	case ErrFileWrite:
		return "Failed to write file."
	case ErrFileInvalid:
		return "File format is invalid."

	// Network related
	case ErrNetworkTimeout:
		return "Network timeout occurred. Please check connection and try again."
	case ErrNetworkUnavailable:
		return "Cannot connect to network. Please check internet connection."
	case ErrDNSResolution:
		return "DNS resolution failed. Please check network settings."

	// System errors
	case ErrInternal:
		return "Internal error occurred. Please check logs for details."
	case ErrNotSupported:
		return "This operation is not supported."
	case ErrUnknown:
		return "Unknown error occurred."
	case ErrTimeout:
		return "Operation timed out."
	case ErrInvalidInput:
		return "Invalid input value."

	default:
		return e.Message
	}
}

// Wrapf wraps an error with formatted message
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf(format, args...)
	if envyErr, ok := err.(*EnvyError); ok {
		return &EnvyError{
			Code:      envyErr.Code,
			Message:   message,
			Details:   envyErr.Details,
			Cause:     err,
			Timestamp: time.Now(),
			Retriable: envyErr.Retriable,
		}
	}
	return &EnvyError{
		Code:      ErrUnknown,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Retriable: false,
	}
}

// ConfigError は設定関連のエラーを作成
func ConfigError(message string) *EnvyError {
	return New(ErrConfigInvalid, message)
}

// ValidationError はバリデーション関連のエラーを作成
func ValidationError(message string) *EnvyError {
	return New(ErrValidationFailed, message)
}

// AWSError はAWS関連のエラーを作成
func AWSError(message string) *EnvyError {
	return New(ErrAWSConnection, message)
}

// FileError はファイル関連のエラーを作成
func FileError(message string) *EnvyError {
	return New(ErrFileInvalid, message)
}

// NetworkError はネットワーク関連のエラーを作成
func NetworkError(message string) *EnvyError {
	return New(ErrNetworkUnavailable, message)
}
