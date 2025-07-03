package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// MockSSMClient is a mock implementation of SSM client
type MockSSMClient struct {
	parameters      map[string]*ssmtypes.Parameter
	mu              sync.RWMutex
	getCallCount    int
	putCallCount    int
	deleteCallCount int
	listCallCount   int
	errors          map[string]error
}

// NewMockSSMClient creates a new mock SSM client
func NewMockSSMClient() *MockSSMClient {
	return &MockSSMClient{
		parameters: make(map[string]*ssmtypes.Parameter),
		errors:     make(map[string]error),
	}
}

// GetParameter mocks the GetParameter method
func (m *MockSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getCallCount++

	// Check for configured error
	if err, ok := m.errors["GetParameter:"+*params.Name]; ok {
		return nil, err
	}

	param, exists := m.parameters[*params.Name]
	if !exists {
		return nil, &ssmtypes.ParameterNotFound{
			Message: aws.String(fmt.Sprintf("Parameter %s not found", *params.Name)),
		}
	}

	// Handle decryption
	if params.WithDecryption != nil && *params.WithDecryption && param.Type == ssmtypes.ParameterTypeSecureString {
		// In real SSM, this would decrypt the value
		// For testing, we'll just return the value as-is
	}

	return &ssm.GetParameterOutput{
		Parameter: param,
	}, nil
}

// PutParameter mocks the PutParameter method
func (m *MockSSMClient) PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.putCallCount++

	// Check for configured error
	if err, ok := m.errors["PutParameter:"+*params.Name]; ok {
		return nil, err
	}

	// Create or update parameter
	version := int64(1)
	if existing, exists := m.parameters[*params.Name]; exists {
		if params.Overwrite == nil || !*params.Overwrite {
			return nil, &ssmtypes.ParameterAlreadyExists{
				Message: aws.String(fmt.Sprintf("Parameter %s already exists", *params.Name)),
			}
		}
		version = existing.Version + 1
	}

	paramType := ssmtypes.ParameterTypeString
	if params.Type != "" {
		paramType = params.Type
	}

	m.parameters[*params.Name] = &ssmtypes.Parameter{
		Name:             params.Name,
		Type:             paramType,
		Value:            params.Value,
		Version:          version,
		LastModifiedDate: aws.Time(time.Now()),
		DataType:         aws.String("text"),
	}

	return &ssm.PutParameterOutput{
		Version: version,
	}, nil
}

// DeleteParameter mocks the DeleteParameter method
func (m *MockSSMClient) DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCallCount++

	// Check for configured error
	if err, ok := m.errors["DeleteParameter:"+*params.Name]; ok {
		return nil, err
	}

	if _, exists := m.parameters[*params.Name]; !exists {
		return nil, &ssmtypes.ParameterNotFound{
			Message: aws.String(fmt.Sprintf("Parameter %s not found", *params.Name)),
		}
	}

	delete(m.parameters, *params.Name)

	return &ssm.DeleteParameterOutput{}, nil
}

// GetParametersByPath mocks the GetParametersByPath method
func (m *MockSSMClient) GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.listCallCount++

	// Check for configured error
	if err, ok := m.errors["GetParametersByPath:"+*params.Path]; ok {
		return nil, err
	}

	var result []ssmtypes.Parameter
	prefix := *params.Path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for name, param := range m.parameters {
		if strings.HasPrefix(name, prefix) {
			// Check if recursive or direct children only
			if params.Recursive != nil && *params.Recursive {
				result = append(result, *param)
			} else {
				// Only direct children
				remainder := strings.TrimPrefix(name, prefix)
				if !strings.Contains(remainder, "/") {
					result = append(result, *param)
				}
			}
		}
	}

	// Handle pagination
	startIndex := 0
	if params.NextToken != nil {
		// Simple pagination: use token as index
		fmt.Sscanf(*params.NextToken, "%d", &startIndex)
	}

	maxResults := 10
	if params.MaxResults != nil {
		maxResults = int(*params.MaxResults)
	}

	endIndex := startIndex + maxResults
	if endIndex > len(result) {
		endIndex = len(result)
	}

	var nextToken *string
	if endIndex < len(result) {
		nextToken = aws.String(fmt.Sprintf("%d", endIndex))
	}

	return &ssm.GetParametersByPathOutput{
		Parameters: result[startIndex:endIndex],
		NextToken:  nextToken,
	}, nil
}

// Helper methods for testing

// SetParameter sets a parameter in the mock
func (m *MockSSMClient) SetParameter(name, value string, paramType ssmtypes.ParameterType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.parameters[name] = &ssmtypes.Parameter{
		Name:             aws.String(name),
		Type:             paramType,
		Value:            aws.String(value),
		Version:          1,
		LastModifiedDate: aws.Time(time.Now()),
		DataType:         aws.String("text"),
	}
}

// SetError configures an error for a specific operation
func (m *MockSSMClient) SetError(operation, name string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors[operation+":"+name] = err
}

// ClearErrors clears all configured errors
func (m *MockSSMClient) ClearErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors = make(map[string]error)
}

// GetCallCounts returns the call counts for each operation
func (m *MockSSMClient) GetCallCounts() (get, put, delete, list int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getCallCount, m.putCallCount, m.deleteCallCount, m.listCallCount
}

// Reset resets the mock state
func (m *MockSSMClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.parameters = make(map[string]*ssmtypes.Parameter)
	m.errors = make(map[string]error)
	m.getCallCount = 0
	m.putCallCount = 0
	m.deleteCallCount = 0
	m.listCallCount = 0
}

// MockSecretsManagerClient is a mock implementation of Secrets Manager client
type MockSecretsManagerClient struct {
	secrets         map[string]*types.SecretVersionsListEntry
	secretValues    map[string]string
	mu              sync.RWMutex
	getCallCount    int
	createCallCount int
	updateCallCount int
	deleteCallCount int
	listCallCount   int
	errors          map[string]error
}

// NewMockSecretsManagerClient creates a new mock Secrets Manager client
func NewMockSecretsManagerClient() *MockSecretsManagerClient {
	return &MockSecretsManagerClient{
		secrets:      make(map[string]*types.SecretVersionsListEntry),
		secretValues: make(map[string]string),
		errors:       make(map[string]error),
	}
}

// GetSecretValue mocks the GetSecretValue method
func (m *MockSecretsManagerClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.getCallCount++

	// Check for configured error
	if err, ok := m.errors["GetSecretValue:"+*params.SecretId]; ok {
		return nil, err
	}

	value, exists := m.secretValues[*params.SecretId]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secret %s not found", *params.SecretId)),
		}
	}

	return &secretsmanager.GetSecretValueOutput{
		ARN:          aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", *params.SecretId)),
		Name:         params.SecretId,
		SecretString: aws.String(value),
		VersionId:    aws.String("AWSCURRENT"),
		CreatedDate:  aws.Time(time.Now()),
	}, nil
}

// CreateSecret mocks the CreateSecret method
func (m *MockSecretsManagerClient) CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.createCallCount++

	// Check for configured error
	if err, ok := m.errors["CreateSecret:"+*params.Name]; ok {
		return nil, err
	}

	if _, exists := m.secretValues[*params.Name]; exists {
		return nil, &types.ResourceExistsException{
			Message: aws.String(fmt.Sprintf("Secret %s already exists", *params.Name)),
		}
	}

	m.secrets[*params.Name] = &types.SecretVersionsListEntry{
		VersionId:   aws.String("AWSCURRENT"),
		CreatedDate: aws.Time(time.Now()),
	}

	if params.SecretString != nil {
		m.secretValues[*params.Name] = *params.SecretString
	}

	return &secretsmanager.CreateSecretOutput{
		ARN:       aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", *params.Name)),
		Name:      params.Name,
		VersionId: aws.String("AWSCURRENT"),
	}, nil
}

// UpdateSecret mocks the UpdateSecret method
func (m *MockSecretsManagerClient) UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateCallCount++

	// Check for configured error
	if err, ok := m.errors["UpdateSecret:"+*params.SecretId]; ok {
		return nil, err
	}

	if _, exists := m.secretValues[*params.SecretId]; !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secret %s not found", *params.SecretId)),
		}
	}

	if params.SecretString != nil {
		m.secretValues[*params.SecretId] = *params.SecretString
	}

	return &secretsmanager.UpdateSecretOutput{
		ARN:       aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", *params.SecretId)),
		Name:      params.SecretId,
		VersionId: aws.String("AWSCURRENT"),
	}, nil
}

// DeleteSecret mocks the DeleteSecret method
func (m *MockSecretsManagerClient) DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCallCount++

	// Check for configured error
	if err, ok := m.errors["DeleteSecret:"+*params.SecretId]; ok {
		return nil, err
	}

	if _, exists := m.secretValues[*params.SecretId]; !exists {
		return nil, &types.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("Secret %s not found", *params.SecretId)),
		}
	}

	delete(m.secrets, *params.SecretId)
	delete(m.secretValues, *params.SecretId)

	return &secretsmanager.DeleteSecretOutput{
		ARN:          aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", *params.SecretId)),
		Name:         params.SecretId,
		DeletionDate: aws.Time(time.Now().Add(30 * 24 * time.Hour)), // 30 days recovery window
	}, nil
}

// ListSecrets mocks the ListSecrets method
func (m *MockSecretsManagerClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.listCallCount++

	// Check for configured error
	if err, ok := m.errors["ListSecrets"]; ok {
		return nil, err
	}

	var secretList []types.SecretListEntry
	for name := range m.secrets {
		secretList = append(secretList, types.SecretListEntry{
			ARN:         aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", name)),
			Name:        aws.String(name),
			CreatedDate: aws.Time(time.Now()),
		})
	}

	return &secretsmanager.ListSecretsOutput{
		SecretList: secretList,
	}, nil
}

// Helper methods for testing

// SetSecret sets a secret in the mock
func (m *MockSecretsManagerClient) SetSecret(name, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.secrets[name] = &types.SecretVersionsListEntry{
		VersionId:   aws.String("AWSCURRENT"),
		CreatedDate: aws.Time(time.Now()),
	}
	m.secretValues[name] = value
}

// SetError configures an error for a specific operation
func (m *MockSecretsManagerClient) SetError(operation, name string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := operation
	if name != "" {
		key += ":" + name
	}
	m.errors[key] = err
}

// ClearErrors clears all configured errors
func (m *MockSecretsManagerClient) ClearErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors = make(map[string]error)
}

// GetCallCounts returns the call counts for each operation
func (m *MockSecretsManagerClient) GetCallCounts() (get, create, update, delete, list int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getCallCount, m.createCallCount, m.updateCallCount, m.deleteCallCount, m.listCallCount
}

// Reset resets the mock state
func (m *MockSecretsManagerClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.secrets = make(map[string]*types.SecretVersionsListEntry)
	m.secretValues = make(map[string]string)
	m.errors = make(map[string]error)
	m.getCallCount = 0
	m.createCallCount = 0
	m.updateCallCount = 0
	m.deleteCallCount = 0
	m.listCallCount = 0
}

// ResponseBuilder helps build AWS responses for testing
type ResponseBuilder struct{}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{}
}

// SSMParameter builds an SSM parameter
func (b *ResponseBuilder) SSMParameter(name, value string, paramType ssmtypes.ParameterType) *ssmtypes.Parameter {
	return &ssmtypes.Parameter{
		Name:             aws.String(name),
		Type:             paramType,
		Value:            aws.String(value),
		Version:          1,
		LastModifiedDate: aws.Time(time.Now()),
		DataType:         aws.String("text"),
	}
}

// Secret builds a Secrets Manager secret
func (b *ResponseBuilder) Secret(name, value string) types.SecretListEntry {
	return types.SecretListEntry{
		ARN:         aws.String(fmt.Sprintf("arn:aws:secretsmanager:us-east-1:123456789012:secret:%s", name)),
		Name:        aws.String(name),
		CreatedDate: aws.Time(time.Now()),
	}
}

// Error builders for common AWS errors

// ParameterNotFoundError creates a parameter not found error
func ParameterNotFoundError(name string) error {
	return &ssmtypes.ParameterNotFound{
		Message: aws.String(fmt.Sprintf("Parameter %s not found", name)),
	}
}

// ParameterAlreadyExistsError creates a parameter already exists error
func ParameterAlreadyExistsError(name string) error {
	return &ssmtypes.ParameterAlreadyExists{
		Message: aws.String(fmt.Sprintf("Parameter %s already exists", name)),
	}
}

// SecretNotFoundError creates a secret not found error
func SecretNotFoundError(name string) error {
	return &types.ResourceNotFoundException{
		Message: aws.String(fmt.Sprintf("Secret %s not found", name)),
	}
}

// SecretAlreadyExistsError creates a secret already exists error
func SecretAlreadyExistsError(name string) error {
	return &types.ResourceExistsException{
		Message: aws.String(fmt.Sprintf("Secret %s already exists", name)),
	}
}
