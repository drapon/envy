package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
)

// Common error types
var (
	ErrParameterNotFound      = errors.New("parameter not found")
	ErrSecretNotFound         = errors.New("secret not found")
	ErrAccessDenied           = errors.New("access denied")
	ErrInvalidParameter       = errors.New("invalid parameter")
	ErrParameterAlreadyExists = errors.New("parameter already exists")
	ErrSecretAlreadyExists    = errors.New("secret already exists")
	ErrRateLimitExceeded      = errors.New("rate limit exceeded")
	ErrInvalidRequest         = errors.New("invalid request")
)

// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific AWS errors
	var ssmNotFound *ssmtypes.ParameterNotFound
	var secretNotFound *types.ResourceNotFoundException

	return errors.As(err, &ssmNotFound) ||
		errors.As(err, &secretNotFound) ||
		errors.Is(err, ErrParameterNotFound) ||
		errors.Is(err, ErrSecretNotFound)
}

// IsAccessDeniedError checks if the error is an access denied error
func IsAccessDeniedError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common access denied patterns
	errStr := err.Error()
	return strings.Contains(errStr, "AccessDenied") ||
		strings.Contains(errStr, "UnauthorizedOperation") ||
		strings.Contains(errStr, "is not authorized to perform") ||
		errors.Is(err, ErrAccessDenied)
}

// IsAlreadyExistsError checks if the error indicates a resource already exists
func IsAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}

	var ssmExists *ssmtypes.ParameterAlreadyExists
	var secretExists *types.ResourceExistsException

	return errors.As(err, &ssmExists) ||
		errors.As(err, &secretExists) ||
		errors.Is(err, ErrParameterAlreadyExists) ||
		errors.Is(err, ErrSecretAlreadyExists)
}

// IsRateLimitError checks if the error is due to rate limiting
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "Throttling") ||
		strings.Contains(errStr, "Rate exceeded") ||
		strings.Contains(errStr, "TooManyRequestsException") ||
		errors.Is(err, ErrRateLimitExceeded)
}

// WrapAWSError wraps AWS errors with more context
func WrapAWSError(err error, operation string, resource string) error {
	if err == nil {
		return nil
	}

	// Extract operation error if available
	var opErr *smithy.OperationError
	if errors.As(err, &opErr) {
		return fmt.Errorf("%s failed for %s: %w", operation, resource, opErr.Err)
	}

	// Map specific AWS errors to our errors
	if IsNotFoundError(err) {
		if strings.Contains(operation, "parameter") {
			return fmt.Errorf("%s failed for %s: %w", operation, resource, ErrParameterNotFound)
		}
		return fmt.Errorf("%s failed for %s: %w", operation, resource, ErrSecretNotFound)
	}

	if IsAccessDeniedError(err) {
		return fmt.Errorf("%s failed for %s: %w (check AWS credentials and IAM permissions)", operation, resource, ErrAccessDenied)
	}

	if IsAlreadyExistsError(err) {
		if strings.Contains(operation, "parameter") {
			return fmt.Errorf("%s failed for %s: %w", operation, resource, ErrParameterAlreadyExists)
		}
		return fmt.Errorf("%s failed for %s: %w", operation, resource, ErrSecretAlreadyExists)
	}

	if IsRateLimitError(err) {
		return fmt.Errorf("%s failed for %s: %w (please retry after a moment)", operation, resource, ErrRateLimitExceeded)
	}

	// Default wrapping
	return fmt.Errorf("%s failed for %s: %w", operation, resource, err)
}

// ExtractAWSErrorCode extracts the AWS error code from an error
func ExtractAWSErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}

	return ""
}

// FormatError formats an error for user display
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	// Special formatting for known errors
	switch {
	case IsNotFoundError(err):
		return "Resource not found. Please check the name and try again."
	case IsAccessDeniedError(err):
		return "Access denied. Please check your AWS credentials and IAM permissions."
	case IsAlreadyExistsError(err):
		return "Resource already exists. Use --force to overwrite."
	case IsRateLimitError(err):
		return "Rate limit exceeded. Please wait a moment and try again."
	default:
		return err.Error()
	}
}
