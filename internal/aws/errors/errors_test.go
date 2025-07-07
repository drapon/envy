package errors

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "parameter not found error",
			err:  &types.ParameterNotFound{},
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFoundError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsAccessDeniedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "smithy error with AccessDenied code",
			err: &smithy.GenericAPIError{
				Code: "AccessDenied",
			},
			want: true,
		},
		{
			name: "smithy error with AccessDeniedException code",
			err: &smithy.GenericAPIError{
				Code: "AccessDeniedException",
			},
			want: true,
		},
		{
			name: "smithy error with UnauthorizedOperation code",
			err: &smithy.GenericAPIError{
				Code: "UnauthorizedOperation",
			},
			want: true,
		},
		{
			name: "smithy error with other code",
			err: &smithy.GenericAPIError{
				Code: "ValidationException",
			},
			want: false,
		},
		{
			name: "non-smithy error",
			err:  errors.New("access denied"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAccessDeniedError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "throttling exception",
			err: &smithy.GenericAPIError{
				Code: "ThrottlingException",
			},
			want: true,
		},
		{
			name: "request limit exceeded",
			err: &smithy.GenericAPIError{
				Code: "RequestLimitExceeded",
			},
			want: true,
		},
		{
			name: "too many requests",
			err: &smithy.GenericAPIError{
				Code: "TooManyRequestsException",
			},
			want: true,
		},
		{
			name: "other error",
			err: &smithy.GenericAPIError{
				Code: "ValidationException",
			},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRateLimitError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractAWSErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "smithy error with code",
			err: &smithy.GenericAPIError{
				Code: "ValidationException",
			},
			want: "ValidationException",
		},
		{
			name: "non-smithy error",
			err:  errors.New("some error"),
			want: "",
		},
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAWSErrorCode(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}