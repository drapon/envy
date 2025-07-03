package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/mock"
)

// MockAWSManager provides a comprehensive mock for AWS operations
type MockAWSManager struct {
	mock.Mock
	ssmClient    *MockSSMClient
	secretsClient *MockSecretsManagerClient
	errorInjector *MockErrorInjector
	mu           sync.RWMutex
}

// NewMockAWSManager creates a new mock AWS manager
func NewMockAWSManager() *MockAWSManager {
	return &MockAWSManager{
		ssmClient:     NewMockSSMClient(),
		secretsClient: NewMockSecretsManagerClient(),
		errorInjector: NewMockErrorInjector(),
	}
}

// GetSSMClient returns the mock SSM client
func (m *MockAWSManager) GetSSMClient() *MockSSMClient {
	return m.ssmClient
}

// GetSecretsManagerClient returns the mock Secrets Manager client
func (m *MockAWSManager) GetSecretsManagerClient() *MockSecretsManagerClient {
	return m.secretsClient
}

// GetErrorInjector returns the error injector for testing error conditions
func (m *MockAWSManager) GetErrorInjector() *MockErrorInjector {
	return m.errorInjector
}

// SetupParameterStore sets up parameter store with test data
func (m *MockAWSManager) SetupParameterStore(parameters map[string]ParameterData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ssmClient.ClearParameters()
	
	for name, data := range parameters {
		paramType := ssmtypes.ParameterTypeString
		if data.Secure {
			paramType = ssmtypes.ParameterTypeSecureString
		}
		
		m.ssmClient.SetParameter(name, data.Value, paramType)
	}
}

// SetupSecretsManager sets up secrets manager with test data
func (m *MockAWSManager) SetupSecretsManager(secrets map[string]SecretData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.secretsClient.ClearSecrets()
	
	for name, data := range secrets {
		var secretValue string
		if data.KeyValue != nil {
			// JSON format for key-value pairs
			jsonBytes, _ := json.Marshal(data.KeyValue)
			secretValue = string(jsonBytes)
		} else {
			secretValue = data.Value
		}
		
		m.secretsClient.SetSecret(name, secretValue)
	}
}

// ParameterData represents parameter store data for testing
type ParameterData struct {
	Value       string
	Secure      bool
	Description string
	Tags        map[string]string
}

// SecretData represents secrets manager data for testing
type SecretData struct {
	Value       string
	KeyValue    map[string]string
	Description string
	Tags        map[string]string
}

// MockParameterStoreClient provides advanced parameter store mocking
type MockParameterStoreClient struct {
	MockSSMClient
	latencySimulation time.Duration
	failureRate       float64
	callCount         map[string]int
	mu                sync.RWMutex
}

// NewMockParameterStoreClient creates a new mock parameter store client
func NewMockParameterStoreClient() *MockParameterStoreClient {
	return &MockParameterStoreClient{
		MockSSMClient: *NewMockSSMClient(),
		callCount:     make(map[string]int),
	}
}

// SetLatencySimulation sets artificial latency for testing
func (m *MockParameterStoreClient) SetLatencySimulation(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencySimulation = latency
}

// SetFailureRate sets a failure rate for testing error conditions
func (m *MockParameterStoreClient) SetFailureRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureRate = rate
}

// GetCallCount returns the number of calls for a specific operation
func (m *MockParameterStoreClient) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[operation]
}

// simulateLatency simulates network latency
func (m *MockParameterStoreClient) simulateLatency() {
	if m.latencySimulation > 0 {
		time.Sleep(m.latencySimulation)
	}
}

// shouldFail determines if an operation should fail based on failure rate
func (m *MockParameterStoreClient) shouldFail() bool {
	// Simple failure simulation - in practice you might want more sophisticated logic
	return false // Simplified for this example
}

// incrementCallCount increments the call count for an operation
func (m *MockParameterStoreClient) incrementCallCount(operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount[operation]++
}

// GetParameter overrides the base implementation with additional features
func (m *MockParameterStoreClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	m.incrementCallCount("GetParameter")
	m.simulateLatency()
	
	if m.shouldFail() {
		return nil, fmt.Errorf("simulated failure")
	}
	
	return m.MockSSMClient.GetParameter(ctx, params, optFns...)
}

// MockSecretsManagerAdvanced provides advanced secrets manager mocking
type MockSecretsManagerAdvanced struct {
	MockSecretsManagerClient
	latencySimulation time.Duration
	failureRate       float64
	callCount         map[string]int
	secretVersions    map[string]map[string]string // secret -> version -> value
	mu                sync.RWMutex
}

// NewMockSecretsManagerAdvanced creates a new advanced mock secrets manager client
func NewMockSecretsManagerAdvanced() *MockSecretsManagerAdvanced {
	return &MockSecretsManagerAdvanced{
		MockSecretsManagerClient: *NewMockSecretsManagerClient(),
		callCount:                make(map[string]int),
		secretVersions:           make(map[string]map[string]string),
	}
}

// SetLatencySimulation sets artificial latency for testing
func (m *MockSecretsManagerAdvanced) SetLatencySimulation(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencySimulation = latency
}

// SetSecretVersion sets a specific version of a secret
func (m *MockSecretsManagerAdvanced) SetSecretVersion(secretName, version, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.secretVersions[secretName] == nil {
		m.secretVersions[secretName] = make(map[string]string)
	}
	m.secretVersions[secretName][version] = value
}

// GetSecretValue overrides the base implementation with version support
func (m *MockSecretsManagerAdvanced) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callCount["GetSecretValue"]++
	
	if m.latencySimulation > 0 {
		time.Sleep(m.latencySimulation)
	}
	
	secretName := *params.SecretId
	
	// Check if specific version is requested
	if params.VersionId != nil {
		if versions, exists := m.secretVersions[secretName]; exists {
			if value, versionExists := versions[*params.VersionId]; versionExists {
				return &secretsmanager.GetSecretValueOutput{
					SecretString: &value,
					Name:         params.SecretId,
					VersionId:    params.VersionId,
				}, nil
			}
		}
		return nil, &smtypes.ResourceNotFoundException{
			Message: aws.String("Version not found"),
		}
	}
	
	// Fall back to base implementation for current version
	return m.MockSecretsManagerClient.GetSecretValue(ctx, params, optFns...)
}

// TestScenario represents a test scenario with specific setup
type TestScenario struct {
	Name                string
	ParameterStoreData  map[string]ParameterData
	SecretsManagerData  map[string]SecretData
	ErrorConditions     map[string]error
	LatencySimulation   time.Duration
	ExpectedResults     map[string]interface{}
}

// ScenarioManager manages test scenarios
type ScenarioManager struct {
	scenarios map[string]*TestScenario
	current   *TestScenario
	mu        sync.RWMutex
}

// NewScenarioManager creates a new scenario manager
func NewScenarioManager() *ScenarioManager {
	return &ScenarioManager{
		scenarios: make(map[string]*TestScenario),
	}
}

// AddScenario adds a test scenario
func (sm *ScenarioManager) AddScenario(scenario *TestScenario) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.scenarios[scenario.Name] = scenario
}

// LoadScenario loads a specific test scenario
func (sm *ScenarioManager) LoadScenario(name string, awsManager *MockAWSManager) error {
	sm.mu.RLock()
	scenario, exists := sm.scenarios[name]
	sm.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("scenario %s not found", name)
	}
	
	sm.mu.Lock()
	sm.current = scenario
	sm.mu.Unlock()
	
	// Setup AWS services with scenario data
	if scenario.ParameterStoreData != nil {
		awsManager.SetupParameterStore(scenario.ParameterStoreData)
	}
	
	if scenario.SecretsManagerData != nil {
		awsManager.SetupSecretsManager(scenario.SecretsManagerData)
	}
	
	// Setup error conditions
	for operation, err := range scenario.ErrorConditions {
		awsManager.GetErrorInjector().InjectError(operation, err)
	}
	
	return nil
}

// GetCurrentScenario returns the current scenario
func (sm *ScenarioManager) GetCurrentScenario() *TestScenario {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// CreateCommonScenarios creates commonly used test scenarios
func CreateCommonScenarios() *ScenarioManager {
	sm := NewScenarioManager()
	
	// Basic success scenario
	sm.AddScenario(&TestScenario{
		Name: "basic_success",
		ParameterStoreData: map[string]ParameterData{
			"/myapp/dev/APP_NAME":    {Value: "test-app", Secure: false},
			"/myapp/dev/DEBUG":       {Value: "true", Secure: false},
			"/myapp/dev/API_KEY":     {Value: "secret-key", Secure: true},
			"/myapp/dev/DATABASE_URL": {Value: "postgres://localhost/db", Secure: false},
		},
		SecretsManagerData: map[string]SecretData{
			"myapp-dev-secrets": {
				KeyValue: map[string]string{
					"DATABASE_PASSWORD": "secret-password",
					"JWT_SECRET":        "jwt-secret-key",
				},
			},
		},
	})
	
	// Empty environment scenario
	sm.AddScenario(&TestScenario{
		Name:               "empty_environment",
		ParameterStoreData: map[string]ParameterData{},
		SecretsManagerData: map[string]SecretData{},
	})
	
	// Large dataset scenario
	largeParameterData := make(map[string]ParameterData)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("/myapp/large/VAR_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, strings.Repeat("x", 100))
		largeParameterData[key] = ParameterData{Value: value, Secure: false}
	}
	
	sm.AddScenario(&TestScenario{
		Name:               "large_dataset",
		ParameterStoreData: largeParameterData,
	})
	
	// Error conditions scenario
	sm.AddScenario(&TestScenario{
		Name: "error_conditions",
		ParameterStoreData: map[string]ParameterData{
			"/myapp/error/GOOD_VAR": {Value: "good-value", Secure: false},
		},
		ErrorConditions: map[string]error{
			"PutParameter":  fmt.Errorf("access denied"),
			"GetParameter":  fmt.Errorf("parameter not found"),
			"CreateSecret": fmt.Errorf("secret already exists"),
		},
	})
	
	// Network latency scenario
	sm.AddScenario(&TestScenario{
		Name: "high_latency",
		ParameterStoreData: map[string]ParameterData{
			"/myapp/latency/VAR1": {Value: "value1", Secure: false},
			"/myapp/latency/VAR2": {Value: "value2", Secure: false},
		},
		LatencySimulation: 100 * time.Millisecond,
	})
	
	// Mixed service types scenario
	sm.AddScenario(&TestScenario{
		Name: "mixed_services",
		ParameterStoreData: map[string]ParameterData{
			"/myapp/mixed/PUBLIC_VAR":  {Value: "public-value", Secure: false},
			"/myapp/mixed/CONFIG_VAR":  {Value: "config-value", Secure: false},
		},
		SecretsManagerData: map[string]SecretData{
			"myapp-mixed-secrets": {
				KeyValue: map[string]string{
					"PRIVATE_KEY":       "private-key-content",
					"DATABASE_PASSWORD": "db-password",
					"API_SECRET":        "api-secret-key",
				},
			},
		},
	})
	
	return sm
}

// TestDataValidator provides validation for test data
type TestDataValidator struct{}

// ValidateParameterStoreData validates parameter store test data
func (v *TestDataValidator) ValidateParameterStoreData(data map[string]ParameterData) error {
	for key, param := range data {
		if key == "" {
			return fmt.Errorf("parameter key cannot be empty")
		}
		
		if !strings.HasPrefix(key, "/") {
			return fmt.Errorf("parameter key must start with '/': %s", key)
		}
		
		if param.Value == "" && !param.Secure {
			// Non-secure parameters can be empty, but warn about it
			// This is just a validation example
		}
		
		if param.Secure && len(param.Value) < 8 {
			return fmt.Errorf("secure parameter should have minimum length: %s", key)
		}
	}
	
	return nil
}

// ValidateSecretsManagerData validates secrets manager test data
func (v *TestDataValidator) ValidateSecretsManagerData(data map[string]SecretData) error {
	for name, secret := range data {
		if name == "" {
			return fmt.Errorf("secret name cannot be empty")
		}
		
		if secret.Value == "" && secret.KeyValue == nil {
			return fmt.Errorf("secret must have either Value or KeyValue: %s", name)
		}
		
		if secret.Value != "" && secret.KeyValue != nil {
			return fmt.Errorf("secret cannot have both Value and KeyValue: %s", name)
		}
		
		if secret.KeyValue != nil {
			for key, value := range secret.KeyValue {
				if key == "" {
					return fmt.Errorf("secret key cannot be empty in %s", name)
				}
				if value == "" {
					return fmt.Errorf("secret value cannot be empty for key %s in %s", key, name)
				}
			}
		}
	}
	
	return nil
}

// PerformanceMetrics tracks performance metrics during testing
type PerformanceMetrics struct {
	OperationCounts map[string]int
	TotalDuration   map[string]time.Duration
	MaxDuration     map[string]time.Duration
	MinDuration     map[string]time.Duration
	ErrorCounts     map[string]int
	mu              sync.RWMutex
}

// NewPerformanceMetrics creates a new performance metrics tracker
func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		OperationCounts: make(map[string]int),
		TotalDuration:   make(map[string]time.Duration),
		MaxDuration:     make(map[string]time.Duration),
		MinDuration:     make(map[string]time.Duration),
		ErrorCounts:     make(map[string]int),
	}
}

// RecordOperation records metrics for an operation
func (pm *PerformanceMetrics) RecordOperation(operation string, duration time.Duration, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.OperationCounts[operation]++
	pm.TotalDuration[operation] += duration
	
	if pm.MaxDuration[operation] < duration {
		pm.MaxDuration[operation] = duration
	}
	
	if pm.MinDuration[operation] == 0 || pm.MinDuration[operation] > duration {
		pm.MinDuration[operation] = duration
	}
	
	if !success {
		pm.ErrorCounts[operation]++
	}
}

// GetAverageDuration returns the average duration for an operation
func (pm *PerformanceMetrics) GetAverageDuration(operation string) time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	count := pm.OperationCounts[operation]
	if count == 0 {
		return 0
	}
	
	return pm.TotalDuration[operation] / time.Duration(count)
}

// GetErrorRate returns the error rate for an operation
func (pm *PerformanceMetrics) GetErrorRate(operation string) float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	count := pm.OperationCounts[operation]
	if count == 0 {
		return 0
	}
	
	return float64(pm.ErrorCounts[operation]) / float64(count)
}

// Reset resets all metrics
func (pm *PerformanceMetrics) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.OperationCounts = make(map[string]int)
	pm.TotalDuration = make(map[string]time.Duration)
	pm.MaxDuration = make(map[string]time.Duration)
	pm.MinDuration = make(map[string]time.Duration)
	pm.ErrorCounts = make(map[string]int)
}