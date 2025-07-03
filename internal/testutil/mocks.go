package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/mock"
)

//go:generate mockgen -source=../../aws/client/client.go -destination=mocks_aws_client.go -package=testutil

// MockSSMClient is a mock implementation of SSM client
type MockSSMClient struct {
	mock.Mock
	parameters map[string]*ssmtypes.Parameter
	mu         sync.RWMutex
}

// NewMockSSMClient creates a new mock SSM client
func NewMockSSMClient() *MockSSMClient {
	return &MockSSMClient{
		parameters: make(map[string]*ssmtypes.Parameter),
	}
}

// GetParameter mocks GetParameter operation
func (m *MockSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if param, exists := m.parameters[*params.Name]; exists {
		return &ssm.GetParameterOutput{
			Parameter: param,
		}, args.Error(1)
	}
	
	// Return ParameterNotFound error
	return nil, &ssmtypes.ParameterNotFound{
		Message: stringPtr("Parameter not found: " + *params.Name),
	}
}

// GetParameters mocks GetParameters operation
func (m *MockSSMClient) GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var parameters []ssmtypes.Parameter
	var invalidParameters []string
	
	for _, name := range params.Names {
		if param, exists := m.parameters[name]; exists {
			parameters = append(parameters, *param)
		} else {
			invalidParameters = append(invalidParameters, name)
		}
	}
	
	return &ssm.GetParametersOutput{
		Parameters:        parameters,
		InvalidParameters: invalidParameters,
	}, args.Error(1)
}

// GetParametersByPath mocks GetParametersByPath operation
func (m *MockSSMClient) GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var parameters []ssmtypes.Parameter
	path := *params.Path
	
	for name, param := range m.parameters {
		if strings.HasPrefix(name, path) {
			parameters = append(parameters, *param)
		}
	}
	
	return &ssm.GetParametersByPathOutput{
		Parameters: parameters,
	}, args.Error(1)
}

// PutParameter mocks PutParameter operation
func (m *MockSSMClient) PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	name := *params.Name
	
	// Check if parameter exists and overwrite is not allowed
	if _, exists := m.parameters[name]; exists && (params.Overwrite == nil || !*params.Overwrite) {
		return nil, &ssmtypes.ParameterAlreadyExists{
			Message: stringPtr("Parameter already exists: " + name),
		}
	}
	
	// Store parameter
	m.parameters[name] = &ssmtypes.Parameter{
		Name:  params.Name,
		Value: params.Value,
		Type:  params.Type,
	}
	
	return &ssm.PutParameterOutput{
		Version: int64(1),
	}, args.Error(1)
}

// DeleteParameter mocks DeleteParameter operation
func (m *MockSSMClient) DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	name := *params.Name
	if _, exists := m.parameters[name]; !exists {
		return nil, &ssmtypes.ParameterNotFound{
			Message: stringPtr("Parameter not found: " + name),
		}
	}
	
	delete(m.parameters, name)
	
	return &ssm.DeleteParameterOutput{}, args.Error(1)
}

// DeleteParameters mocks DeleteParameters operation
func (m *MockSSMClient) DeleteParameters(ctx context.Context, params *ssm.DeleteParametersInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParametersOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var deletedParameters []string
	var invalidParameters []string
	
	for _, name := range params.Names {
		if _, exists := m.parameters[name]; exists {
			delete(m.parameters, name)
			deletedParameters = append(deletedParameters, name)
		} else {
			invalidParameters = append(invalidParameters, name)
		}
	}
	
	return &ssm.DeleteParametersOutput{
		DeletedParameters: deletedParameters,
		InvalidParameters: invalidParameters,
	}, args.Error(1)
}

// SetParameter sets a parameter for testing
func (m *MockSSMClient) SetParameter(name, value string, paramType ssmtypes.ParameterType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.parameters[name] = &ssmtypes.Parameter{
		Name:  &name,
		Value: &value,
		Type:  paramType,
	}
}

// ClearParameters clears all parameters
func (m *MockSSMClient) ClearParameters() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.parameters = make(map[string]*ssmtypes.Parameter)
}

// GetParameterCount returns the number of stored parameters
func (m *MockSSMClient) GetParameterCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return len(m.parameters)
}

// MockSecretsManagerClient is a mock implementation of Secrets Manager client
type MockSecretsManagerClient struct {
	mock.Mock
	secrets map[string]*types.SecretListEntry
	values  map[string]string
	mu      sync.RWMutex
}

// NewMockSecretsManagerClient creates a new mock Secrets Manager client
func NewMockSecretsManagerClient() *MockSecretsManagerClient {
	return &MockSecretsManagerClient{
		secrets: make(map[string]*types.SecretListEntry),
		values:  make(map[string]string),
	}
}

// GetSecretValue mocks GetSecretValue operation
func (m *MockSecretsManagerClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	secretId := getSecretId(params.SecretId, params.VersionId, params.VersionStage)
	
	if value, exists := m.values[secretId]; exists {
		return &secretsmanager.GetSecretValueOutput{
			SecretString: &value,
			Name:         params.SecretId,
		}, args.Error(1)
	}
	
	return nil, &types.ResourceNotFoundException{
		Message: stringPtr("Secret not found: " + secretId),
	}
}

// CreateSecret mocks CreateSecret operation
func (m *MockSecretsManagerClient) CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	name := *params.Name
	
	if _, exists := m.secrets[name]; exists {
		return nil, &types.ResourceExistsException{
			Message: stringPtr("Secret already exists: " + name),
		}
	}
	
	m.secrets[name] = &types.SecretListEntry{
		Name: params.Name,
	}
	
	if params.SecretString != nil {
		m.values[name] = *params.SecretString
	}
	
	return &secretsmanager.CreateSecretOutput{
		Name: params.Name,
	}, args.Error(1)
}

// UpdateSecret mocks UpdateSecret operation
func (m *MockSecretsManagerClient) UpdateSecret(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	secretId := *params.SecretId
	
	if _, exists := m.secrets[secretId]; !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr("Secret not found: " + secretId),
		}
	}
	
	if params.SecretString != nil {
		m.values[secretId] = *params.SecretString
	}
	
	return &secretsmanager.UpdateSecretOutput{
		Name: params.SecretId,
	}, args.Error(1)
}

// DeleteSecret mocks DeleteSecret operation
func (m *MockSecretsManagerClient) DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	secretId := *params.SecretId
	
	if _, exists := m.secrets[secretId]; !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr("Secret not found: " + secretId),
		}
	}
	
	delete(m.secrets, secretId)
	delete(m.values, secretId)
	
	return &secretsmanager.DeleteSecretOutput{
		Name: params.SecretId,
	}, args.Error(1)
}

// ListSecrets mocks ListSecrets operation
func (m *MockSecretsManagerClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	args := m.Called(ctx, params, optFns)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var secretList []types.SecretListEntry
	for _, secret := range m.secrets {
		secretList = append(secretList, *secret)
	}
	
	return &secretsmanager.ListSecretsOutput{
		SecretList: secretList,
	}, args.Error(1)
}

// SetSecret sets a secret for testing
func (m *MockSecretsManagerClient) SetSecret(name, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.secrets[name] = &types.SecretListEntry{
		Name: &name,
	}
	m.values[name] = value
}

// ClearSecrets clears all secrets
func (m *MockSecretsManagerClient) ClearSecrets() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.secrets = make(map[string]*types.SecretListEntry)
	m.values = make(map[string]string)
}

// GetSecretCount returns the number of stored secrets
func (m *MockSecretsManagerClient) GetSecretCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return len(m.secrets)
}

// MockFileSystem provides file system mocking for tests
type MockFileSystem struct {
	files map[string]string
	mu    sync.RWMutex
}

// NewMockFileSystem creates a new mock file system
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string]string),
	}
}

// WriteFile mocks writing a file
func (m *MockFileSystem) WriteFile(path, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.files[path] = content
	return nil
}

// ReadFile mocks reading a file
func (m *MockFileSystem) ReadFile(path string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	
	return "", fmt.Errorf("file not found: %s", path)
}

// FileExists checks if a file exists
func (m *MockFileSystem) FileExists(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	_, exists := m.files[path]
	return exists
}

// DeleteFile deletes a file
func (m *MockFileSystem) DeleteFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.files[path]; !exists {
		return fmt.Errorf("file not found: %s", path)
	}
	
	delete(m.files, path)
	return nil
}

// ListFiles lists all files
func (m *MockFileSystem) ListFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var files []string
	for path := range m.files {
		files = append(files, path)
	}
	return files
}

// Clear clears all files
func (m *MockFileSystem) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.files = make(map[string]string)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func getSecretId(secretId, versionId, versionStage *string) string {
	id := *secretId
	if versionId != nil {
		id += ":" + *versionId
	}
	if versionStage != nil {
		id += ":" + *versionStage
	}
	return id
}

// MockErrorInjector allows injecting errors for testing error handling
type MockErrorInjector struct {
	errors map[string]error
	mu     sync.RWMutex
}

// NewMockErrorInjector creates a new error injector
func NewMockErrorInjector() *MockErrorInjector {
	return &MockErrorInjector{
		errors: make(map[string]error),
	}
}

// InjectError injects an error for a specific operation
func (m *MockErrorInjector) InjectError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.errors[operation] = err
}

// GetError returns the injected error for an operation
func (m *MockErrorInjector) GetError(operation string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.errors[operation]
}

// ClearErrors clears all injected errors
func (m *MockErrorInjector) ClearErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.errors = make(map[string]error)
}

// ShouldError checks if an error should be returned for an operation
func (m *MockErrorInjector) ShouldError(operation string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	_, exists := m.errors[operation]
	return exists
}