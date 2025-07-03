package parameter_store

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/drapon/envy/internal/aws/client"
)

// Store represents a Parameter Store client wrapper
type Store struct {
	client    *client.Client
	ssmClient *ssm.Client
}

// NewStore creates a new Parameter Store client
func NewStore(awsClient *client.Client) *Store {
	return &Store{
		client:    awsClient,
		ssmClient: awsClient.SSM(),
	}
}

// Parameter represents a parameter with metadata
type Parameter struct {
	Name         string
	Value        string
	Type         string
	Version      int64
	LastModified string
	Description  string
}

// GetParameter retrieves a single parameter
func (s *Store) GetParameter(ctx context.Context, name string, withDecryption bool) (*Parameter, error) {
	input := &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(withDecryption),
	}

	result, err := s.ssmClient.GetParameter(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get parameter %s: %w", name, err)
	}

	if result.Parameter == nil {
		return nil, fmt.Errorf("parameter %s not found", name)
	}

	return &Parameter{
		Name:         aws.ToString(result.Parameter.Name),
		Value:        aws.ToString(result.Parameter.Value),
		Type:         string(result.Parameter.Type),
		Version:      result.Parameter.Version,
		LastModified: result.Parameter.LastModifiedDate.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetParametersByPath retrieves all parameters under a specific path
func (s *Store) GetParametersByPath(ctx context.Context, path string, recursive bool, withDecryption bool) ([]*Parameter, error) {
	var parameters []*Parameter
	var nextToken *string

	// Ensure path ends with /
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	for {
		input := &ssm.GetParametersByPathInput{
			Path:           aws.String(path),
			Recursive:      aws.Bool(recursive),
			WithDecryption: aws.Bool(withDecryption),
			NextToken:      nextToken,
			MaxResults:     aws.Int32(10), // AWS allows max 10 for encrypted params
		}

		result, err := s.ssmClient.GetParametersByPath(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get parameters by path %s: %w", path, err)
		}

		for _, param := range result.Parameters {
			parameters = append(parameters, &Parameter{
				Name:         aws.ToString(param.Name),
				Value:        aws.ToString(param.Value),
				Type:         string(param.Type),
				Version:      param.Version,
				LastModified: param.LastModifiedDate.Format("2006-01-02 15:04:05"),
			})
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	return parameters, nil
}

// PutParameter creates or updates a parameter
func (s *Store) PutParameter(ctx context.Context, name, value, description string, paramType string, overwrite bool) error {
	if paramType == "" {
		paramType = "String"
	}

	// Convert string type to AWS type
	var awsType types.ParameterType
	switch paramType {
	case "String":
		awsType = types.ParameterTypeString
	case "SecureString":
		awsType = types.ParameterTypeSecureString
	case "StringList":
		awsType = types.ParameterTypeStringList
	default:
		return fmt.Errorf("invalid parameter type: %s", paramType)
	}

	input := &ssm.PutParameterInput{
		Name:      aws.String(name),
		Value:     aws.String(value),
		Type:      awsType,
		Overwrite: aws.Bool(overwrite),
	}

	if description != "" {
		input.Description = aws.String(description)
	}

	_, err := s.ssmClient.PutParameter(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put parameter %s: %w", name, err)
	}

	return nil
}

// DeleteParameter deletes a parameter
func (s *Store) DeleteParameter(ctx context.Context, name string) error {
	input := &ssm.DeleteParameterInput{
		Name: aws.String(name),
	}

	_, err := s.ssmClient.DeleteParameter(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete parameter %s: %w", name, err)
	}

	return nil
}

// DeleteParametersByPath deletes all parameters under a path
func (s *Store) DeleteParametersByPath(ctx context.Context, path string) error {
	// First, get all parameters under the path
	parameters, err := s.GetParametersByPath(ctx, path, true, false)
	if err != nil {
		return err
	}

	// Delete each parameter
	for _, param := range parameters {
		if err := s.DeleteParameter(ctx, param.Name); err != nil {
			return err
		}
	}

	return nil
}

// ListParameterNames lists all parameter names under a path
func (s *Store) ListParameterNames(ctx context.Context, path string, recursive bool) ([]string, error) {
	parameters, err := s.GetParametersByPath(ctx, path, recursive, false)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(parameters))
	for i, param := range parameters {
		names[i] = param.Name
	}

	return names, nil
}

// BatchPutParameters puts multiple parameters in a batch
func (s *Store) BatchPutParameters(ctx context.Context, parameters map[string]string, pathPrefix string, paramType string, overwrite bool) error {
	for key, value := range parameters {
		name := pathPrefix + key
		if err := s.PutParameter(ctx, name, value, "", paramType, overwrite); err != nil {
			return fmt.Errorf("failed to put parameter %s: %w", key, err)
		}
	}
	return nil
}

// ConvertToEnvVars converts parameters to environment variable format
func (s *Store) ConvertToEnvVars(parameters []*Parameter, stripPrefix string) map[string]string {
	envVars := make(map[string]string)

	for _, param := range parameters {
		key := param.Name

		// Strip prefix if specified
		if stripPrefix != "" && strings.HasPrefix(key, stripPrefix) {
			key = strings.TrimPrefix(key, stripPrefix)
		}

		// Convert path separators to underscores
		key = strings.ReplaceAll(key, "/", "_")

		// Remove leading underscore if present
		key = strings.TrimPrefix(key, "_")

		// Convert to uppercase
		key = strings.ToUpper(key)

		envVars[key] = param.Value
	}

	return envVars
}
