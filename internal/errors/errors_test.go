package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvyError(t *testing.T) {
	t.Run("NewError", func(t *testing.T) {
		err := New(ErrConfigNotFound, "config not found")
		assert.NotNil(t, err)
		assert.Equal(t, ErrConfigNotFound, err.Code)
		assert.Equal(t, "config not found", err.Message)
		assert.NotNil(t, err.Details)
		assert.False(t, err.Retriable)
		assert.NotZero(t, err.Timestamp)
	})

	t.Run("ErrorString", func(t *testing.T) {
		err := New(ErrConfigNotFound, "config not found")
		assert.Equal(t, "[CONFIG_NOT_FOUND] config not found", err.Error())

		err.WithCause(errors.New("underlying error"))
		assert.Equal(t, "[CONFIG_NOT_FOUND] config not found: underlying error", err.Error())
	})

	t.Run("WithCause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := New(ErrConfigNotFound, "config not found").WithCause(cause)
		assert.Equal(t, cause, err.Cause)
		assert.Equal(t, cause, errors.Unwrap(err))
	})

	t.Run("WithDetails", func(t *testing.T) {
		err := New(ErrFileNotFound, "file not found").
			WithDetails("file", "/path/to/file").
			WithDetails("operation", "read")

		assert.Equal(t, "/path/to/file", err.Details["file"])
		assert.Equal(t, "read", err.Details["operation"])
	})

	t.Run("WithRetriable", func(t *testing.T) {
		err := New(ErrAWSRateLimit, "rate limit").WithRetriable(true)
		assert.True(t, err.Retriable)
	})

	t.Run("Is", func(t *testing.T) {
		err1 := New(ErrConfigNotFound, "message 1")
		err2 := New(ErrConfigNotFound, "message 2")
		err3 := New(ErrFileNotFound, "message 3")

		assert.True(t, errors.Is(err1, err2))
		assert.False(t, errors.Is(err1, err3))
	})

	t.Run("UserMessage", func(t *testing.T) {
		tests := []struct {
			name     string
			err      *EnvyError
			expected string
		}{
			{
				name:     "ConfigNotFound",
				err:      New(ErrConfigNotFound, ""),
				expected: "Configuration file not found. Please run 'envy init' to initialize settings.",
			},
			{
				name: "InvalidEnvironment with details",
				err: New(ErrInvalidEnvironment, "").
					WithDetails("environment", "production"),
				expected: "Environment 'production' does not exist. Please specify an environment defined in .envyrc file.",
			},
			{
				name:     "AWSRateLimit",
				err:      New(ErrAWSRateLimit, ""),
				expected: "AWS API rate limit reached. Please wait and try again.",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, tt.err.UserMessage())
			})
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsConfigError", func(t *testing.T) {
		assert.True(t, IsConfigError(New(ErrConfigNotFound, "")))
		assert.True(t, IsConfigError(New(ErrConfigInvalid, "")))
		assert.False(t, IsConfigError(New(ErrFileNotFound, "")))
		assert.False(t, IsConfigError(errors.New("random error")))
	})

	t.Run("IsValidationError", func(t *testing.T) {
		assert.True(t, IsValidationError(New(ErrValidationFailed, "")))
		assert.True(t, IsValidationError(New(ErrInvalidArgument, "")))
		assert.False(t, IsValidationError(New(ErrConfigNotFound, "")))
	})

	t.Run("IsAWSError", func(t *testing.T) {
		assert.True(t, IsAWSError(New(ErrAWSAuth, "")))
		assert.True(t, IsAWSError(New(ErrParameterNotFound, "")))
		assert.False(t, IsAWSError(New(ErrFileNotFound, "")))
	})

	t.Run("IsFileError", func(t *testing.T) {
		assert.True(t, IsFileError(New(ErrFileNotFound, "")))
		assert.True(t, IsFileError(New(ErrFilePermission, "")))
		assert.False(t, IsFileError(New(ErrAWSAuth, "")))
	})

	t.Run("IsNetworkError", func(t *testing.T) {
		assert.True(t, IsNetworkError(New(ErrNetworkTimeout, "")))
		assert.True(t, IsNetworkError(New(ErrNetworkUnavailable, "")))
		assert.True(t, IsNetworkError(errors.New("connection refused")))
		assert.True(t, IsNetworkError(errors.New("Connection timed out")))
		assert.False(t, IsNetworkError(New(ErrConfigNotFound, "")))
	})

	t.Run("IsRetriable", func(t *testing.T) {
		// Explicitly retriable
		assert.True(t, IsRetriable(New(ErrAWSAuth, "").WithRetriable(true)))

		// Implicitly retriable error codes
		assert.True(t, IsRetriable(New(ErrAWSRateLimit, "")))
		assert.True(t, IsRetriable(New(ErrAWSTimeout, "")))
		assert.True(t, IsRetriable(New(ErrNetworkTimeout, "")))

		// Non-retriable
		assert.False(t, IsRetriable(New(ErrConfigNotFound, "")))
		assert.False(t, IsRetriable(nil))
	})

	t.Run("GetErrorCode", func(t *testing.T) {
		assert.Equal(t, ErrConfigNotFound, GetErrorCode(New(ErrConfigNotFound, "")))
		assert.Equal(t, ErrUnknown, GetErrorCode(errors.New("random error")))
	})

	t.Run("GetErrorDetails", func(t *testing.T) {
		err := New(ErrFileNotFound, "").
			WithDetails("file", "test.txt").
			WithDetails("line", 42)

		details := GetErrorDetails(err)
		assert.Equal(t, "test.txt", details["file"])
		assert.Equal(t, 42, details["line"])

		assert.Nil(t, GetErrorDetails(errors.New("random error")))
	})

	t.Run("Wrap", func(t *testing.T) {
		cause := errors.New("underlying error")
		wrapped := Wrap(cause, ErrConfigNotFound, "config error")

		assert.Equal(t, ErrConfigNotFound, wrapped.Code)
		assert.Equal(t, "config error", wrapped.Message)
		assert.Equal(t, cause, wrapped.Cause)

		assert.Nil(t, Wrap(nil, ErrConfigNotFound, "config error"))
	})
}

func TestWrappers(t *testing.T) {
	t.Run("WrapAWSError", func(t *testing.T) {
		tests := []struct {
			name         string
			err          error
			expectedCode ErrorCode
			retriable    bool
		}{
			{
				name:         "AccessDenied",
				err:          errors.New("AccessDenied: User is not authorized"),
				expectedCode: ErrAWSAccessDenied,
				retriable:    false,
			},
			{
				name:         "ParameterNotFound",
				err:          errors.New("ParameterNotFound: Parameter /app/db_host not found"),
				expectedCode: ErrParameterNotFound,
				retriable:    false,
			},
			{
				name:         "Throttling",
				err:          errors.New("ThrottlingException: Rate exceeded"),
				expectedCode: ErrAWSRateLimit,
				retriable:    true,
			},
			{
				name:         "RequestTimeout",
				err:          errors.New("RequestTimeout"),
				expectedCode: ErrAWSTimeout,
				retriable:    true,
			},
			{
				name:         "Generic",
				err:          errors.New("Some AWS error"),
				expectedCode: ErrAWSConnection,
				retriable:    false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				wrapped := WrapAWSError(tt.err, "GetParameter", "/app/test")
				require.NotNil(t, wrapped)
				assert.Equal(t, tt.expectedCode, wrapped.Code)
				assert.Equal(t, tt.retriable, wrapped.Retriable)
				assert.Equal(t, tt.err, wrapped.Cause)
				assert.Equal(t, "GetParameter", wrapped.Details["operation"])
			})
		}
	})

	t.Run("WrapFileError", func(t *testing.T) {
		tests := []struct {
			name         string
			err          error
			expectedCode ErrorCode
		}{
			{
				name:         "FileNotFound",
				err:          errors.New("open test.txt: no such file or directory"),
				expectedCode: ErrFileNotFound,
			},
			{
				name:         "PermissionDenied",
				err:          errors.New("open test.txt: permission denied"),
				expectedCode: ErrFilePermission,
			},
			{
				name:         "Generic",
				err:          errors.New("file error"),
				expectedCode: ErrFileInvalid,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				wrapped := WrapFileError(tt.err, "/path/to/file")
				require.NotNil(t, wrapped)
				assert.Equal(t, tt.expectedCode, wrapped.Code)
				assert.Equal(t, "/path/to/file", wrapped.Details["file"])
			})
		}
	})

	t.Run("WrapNetworkError", func(t *testing.T) {
		tests := []struct {
			name         string
			err          error
			expectedCode ErrorCode
			retriable    bool
		}{
			{
				name:         "Timeout",
				err:          errors.New("context deadline exceeded: timeout"),
				expectedCode: ErrNetworkTimeout,
				retriable:    true,
			},
			{
				name:         "DNSError",
				err:          errors.New("dial tcp: lookup example.com: no such host"),
				expectedCode: ErrDNSResolution,
				retriable:    false,
			},
			{
				name:         "ConnectionRefused",
				err:          errors.New("dial tcp 127.0.0.1:8080: connection refused"),
				expectedCode: ErrNetworkUnavailable,
				retriable:    true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				wrapped := WrapNetworkError(tt.err)
				require.NotNil(t, wrapped)
				assert.Equal(t, tt.expectedCode, wrapped.Code)
				assert.Equal(t, tt.retriable, wrapped.Retriable)
			})
		}
	})
}

func TestErrorAggregator(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		agg := NewAggregator()
		assert.False(t, agg.HasErrors())
		assert.Nil(t, agg.Error())
		assert.Empty(t, agg.Errors())
	})

	t.Run("SingleError", func(t *testing.T) {
		agg := NewAggregator()
		err := errors.New("test error")
		agg.Add(err)

		assert.True(t, agg.HasErrors())
		assert.Equal(t, err, agg.Error())
		assert.Len(t, agg.Errors(), 1)
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		agg := NewAggregator()
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		err3 := errors.New("error 3")

		agg.Add(err1)
		agg.Add(err2)
		agg.Add(err3)
		agg.Add(nil) // Should be ignored

		assert.True(t, agg.HasErrors())
		assert.Len(t, agg.Errors(), 3)

		aggErr := agg.Error()
		require.NotNil(t, aggErr)

		var envyErr *EnvyError
		require.True(t, errors.As(aggErr, &envyErr))
		assert.Equal(t, ErrInternal, envyErr.Code)
		assert.Equal(t, 3, envyErr.Details["count"])
	})

	t.Run("AddWithContext", func(t *testing.T) {
		agg := NewAggregator()
		err := errors.New("test error")
		agg.AddWithContext(err, "failed to process file")

		assert.True(t, agg.HasErrors())
		aggErr := agg.Error()

		var envyErr *EnvyError
		require.True(t, errors.As(aggErr, &envyErr))
		assert.Equal(t, "failed to process file", envyErr.Message)
	})
}
