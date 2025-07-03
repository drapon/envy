package errors

import (
	"github.com/drapon/envy/internal/aws/errors"
)

// AdaptAWSError converts AWS package errors to internal errors
func AdaptAWSError(err error) error {
	if err == nil {
		return nil
	}

	// Check for known AWS error types
	switch err {
	case errors.ErrParameterNotFound:
		return New(ErrParameterNotFound, "Parameter not found")
	case errors.ErrSecretNotFound:
		return New(ErrSecretNotFound, "Secret not found")
	case errors.ErrAccessDenied:
		return New(ErrAWSAccessDenied, "Access denied")
	case errors.ErrParameterAlreadyExists:
		return New(ErrParameterExists, "Parameter already exists")
	case errors.ErrSecretAlreadyExists:
		return New(ErrSecretExists, "Secret already exists")
	case errors.ErrRateLimitExceeded:
		return New(ErrAWSRateLimit, "Rate limit exceeded").WithRetriable(true)
	case errors.ErrInvalidParameter:
		return New(ErrInvalidArgument, "Invalid parameter")
	case errors.ErrInvalidRequest:
		return New(ErrInvalidArgument, "Invalid request")
	}

	// Check error characteristics
	if errors.IsNotFoundError(err) {
		return New(ErrParameterNotFound, "Resource not found").WithCause(err)
	}

	if errors.IsAccessDeniedError(err) {
		return New(ErrAWSAccessDenied, "Access denied").WithCause(err)
	}

	if errors.IsAlreadyExistsError(err) {
		return New(ErrParameterExists, "Resource already exists").WithCause(err)
	}

	if errors.IsRateLimitError(err) {
		return New(ErrAWSRateLimit, "Rate limit exceeded").WithCause(err).WithRetriable(true)
	}

	// Default AWS error
	return New(ErrAWSConnection, "AWS operation failed").WithCause(err)
}

// EnhanceAWSError enhances AWS error with operation context
func EnhanceAWSError(err error, operation string, resource string) error {
	if err == nil {
		return nil
	}

	// First adapt the error
	adaptedErr := AdaptAWSError(err)

	// If it's already an EnvyError, enhance it
	if envyErr, ok := adaptedErr.(*EnvyError); ok {
		return envyErr.
			WithDetails("operation", operation).
			WithDetails("resource", resource)
	}

	// Otherwise wrap it
	return WrapAWSError(err, operation, resource)
}
