package secrets_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/drapon/envy/internal/aws/client"
)

// Manager represents a Secrets Manager client wrapper
type Manager struct {
	client        *client.Client
	secretsClient *secretsmanager.Client
}

// NewManager creates a new Secrets Manager client
func NewManager(awsClient *client.Client) *Manager {
	return &Manager{
		client:        awsClient,
		secretsClient: awsClient.SecretsManager(),
	}
}

// Secret represents a secret with metadata
type Secret struct {
	Name         string
	ARN          string
	Description  string
	Value        string            // For string secrets
	KeyValue     map[string]string // For JSON key-value secrets
	CreatedDate  string
	LastModified string
	VersionId    string
}

// GetSecret retrieves a secret
func (m *Manager) GetSecret(ctx context.Context, name string) (*Secret, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	result, err := m.secretsClient.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", name, err)
	}

	secret := &Secret{
		Name:      aws.ToString(result.Name),
		ARN:       aws.ToString(result.ARN),
		VersionId: aws.ToString(result.VersionId),
	}

	if result.CreatedDate != nil {
		secret.CreatedDate = result.CreatedDate.Format("2006-01-02 15:04:05")
	}

	// Handle secret value
	if result.SecretString != nil {
		secretString := aws.ToString(result.SecretString)

		// Try to parse as JSON
		var keyValue map[string]string
		if err := json.Unmarshal([]byte(secretString), &keyValue); err == nil {
			secret.KeyValue = keyValue
		} else {
			secret.Value = secretString
		}
	}

	return secret, nil
}

// CreateSecret creates a new secret
func (m *Manager) CreateSecret(ctx context.Context, name, description string, value interface{}) error {
	input := &secretsmanager.CreateSecretInput{
		Name: aws.String(name),
	}

	if description != "" {
		input.Description = aws.String(description)
	}

	// Handle different value types
	switch v := value.(type) {
	case string:
		input.SecretString = aws.String(v)
	case map[string]string:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal secret value: %w", err)
		}
		input.SecretString = aws.String(string(jsonBytes))
	default:
		return fmt.Errorf("unsupported secret value type")
	}

	_, err := m.secretsClient.CreateSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create secret %s: %w", name, err)
	}

	return nil
}

// UpdateSecret updates an existing secret
func (m *Manager) UpdateSecret(ctx context.Context, name string, value interface{}) error {
	input := &secretsmanager.UpdateSecretInput{
		SecretId: aws.String(name),
	}

	// Handle different value types
	switch v := value.(type) {
	case string:
		input.SecretString = aws.String(v)
	case map[string]string:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal secret value: %w", err)
		}
		input.SecretString = aws.String(string(jsonBytes))
	default:
		return fmt.Errorf("unsupported secret value type")
	}

	_, err := m.secretsClient.UpdateSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update secret %s: %w", name, err)
	}

	return nil
}

// DeleteSecret deletes a secret
func (m *Manager) DeleteSecret(ctx context.Context, name string, forceDelete bool) error {
	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(name),
		ForceDeleteWithoutRecovery: aws.Bool(forceDelete),
	}

	if !forceDelete {
		// Schedule deletion after 30 days by default
		input.RecoveryWindowInDays = aws.Int64(30)
	}

	_, err := m.secretsClient.DeleteSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s: %w", name, err)
	}

	return nil
}

// ListSecrets lists all secrets with optional filtering
func (m *Manager) ListSecrets(ctx context.Context, namePrefix string) ([]*Secret, error) {
	var secrets []*Secret
	var nextToken *string

	for {
		input := &secretsmanager.ListSecretsInput{
			NextToken:  nextToken,
			MaxResults: aws.Int32(100),
		}

		result, err := m.secretsClient.ListSecrets(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secretEntry := range result.SecretList {
			name := aws.ToString(secretEntry.Name)

			// Filter by prefix if specified
			if namePrefix != "" && !strings.HasPrefix(name, namePrefix) {
				continue
			}

			secret := &Secret{
				Name:        name,
				ARN:         aws.ToString(secretEntry.ARN),
				Description: aws.ToString(secretEntry.Description),
			}

			if secretEntry.CreatedDate != nil {
				secret.CreatedDate = secretEntry.CreatedDate.Format("2006-01-02 15:04:05")
			}
			if secretEntry.LastChangedDate != nil {
				secret.LastModified = secretEntry.LastChangedDate.Format("2006-01-02 15:04:05")
			}

			secrets = append(secrets, secret)
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	return secrets, nil
}

// CreateOrUpdateSecret creates a new secret or updates if it exists
func (m *Manager) CreateOrUpdateSecret(ctx context.Context, name, description string, value interface{}) error {
	// Try to update first
	err := m.UpdateSecret(ctx, name, value)
	if err != nil {
		// Check if secret doesn't exist
		var notFoundErr *types.ResourceNotFoundException
		if ok := isAWSError(err, &notFoundErr); ok {
			// Create new secret
			return m.CreateSecret(ctx, name, description, value)
		}
		return err
	}
	return nil
}

// BatchCreateOrUpdateSecrets creates or updates multiple secrets
func (m *Manager) BatchCreateOrUpdateSecrets(ctx context.Context, secrets map[string]map[string]string, namePrefix, description string) error {
	for name, keyValues := range secrets {
		fullName := namePrefix + name
		if err := m.CreateOrUpdateSecret(ctx, fullName, description, keyValues); err != nil {
			return fmt.Errorf("failed to create/update secret %s: %w", fullName, err)
		}
	}
	return nil
}

// ConvertToEnvVars converts secrets to environment variable format
func (m *Manager) ConvertToEnvVars(secrets []*Secret, stripPrefix string) map[string]string {
	envVars := make(map[string]string)

	for _, secret := range secrets {
		// Handle key-value secrets
		if secret.KeyValue != nil {
			for key, value := range secret.KeyValue {
				envKey := formatEnvKey(key)
				envVars[envKey] = value
			}
		} else if secret.Value != "" {
			// Handle string secrets
			key := secret.Name

			// Strip prefix if specified
			if stripPrefix != "" && strings.HasPrefix(key, stripPrefix) {
				key = strings.TrimPrefix(key, stripPrefix)
			}

			envKey := formatEnvKey(key)
			envVars[envKey] = secret.Value
		}
	}

	return envVars
}

// formatEnvKey formats a key to be a valid environment variable name
func formatEnvKey(key string) string {
	// Convert to uppercase
	key = strings.ToUpper(key)

	// Replace invalid characters with underscores
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, ".", "_")
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, " ", "_")

	// Remove leading numbers
	for len(key) > 0 && key[0] >= '0' && key[0] <= '9' {
		key = key[1:]
	}

	// Remove leading underscores
	key = strings.TrimPrefix(key, "_")

	return key
}

// isAWSError checks if an error is of a specific AWS error type
func isAWSError(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	return errors.As(err, target)
}
