package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/drapon/envy/internal/aws/client"
	"github.com/drapon/envy/internal/aws/errors"
	parameter_store "github.com/drapon/envy/internal/aws/parameter_store"
	secrets_manager "github.com/drapon/envy/internal/aws/secrets_manager"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/memory"
	"github.com/drapon/envy/internal/prompt"
)

// Manager manages AWS operations for envy
type Manager struct {
	client         *client.Client
	paramStore     *parameter_store.Store
	secretsManager *secrets_manager.Manager
	config         *config.Config
}

// GetConfig returns the configuration
func (m *Manager) GetConfig() *config.Config {
	return m.config
}

// NewManager creates a new AWS manager
func NewManager(cfg *config.Config) (*Manager, error) {
	ctx := context.Background()
	
	// Create AWS client
	awsClient, err := client.NewClient(ctx, client.Options{
		Region:  cfg.AWS.Region,
		Profile: cfg.AWS.Profile,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %w", err)
	}

	return &Manager{
		client:         awsClient,
		paramStore:     parameter_store.NewStore(awsClient),
		secretsManager: secrets_manager.NewManager(awsClient),
		config:         cfg,
	}, nil
}

// PushEnvironment pushes environment variables to AWS
func (m *Manager) PushEnvironment(ctx context.Context, envName string, file *env.File, overwrite bool) error {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	// Convert to map using memory pool
	vars, cleanup := file.ToMapWithPool()
	defer cleanup()

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		// Use Secrets Manager
		return m.pushToSecretsManager(ctx, path, vars, overwrite)
	}

	// Use Parameter Store
	return m.pushToParameterStore(ctx, path, vars, overwrite)
}

// PullEnvironment pulls environment variables from AWS
func (m *Manager) PullEnvironment(ctx context.Context, envName string) (*env.File, error) {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	var vars map[string]string

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		// Pull from Secrets Manager
		vars, err = m.pullFromSecretsManager(ctx, path)
	} else {
		// Pull from Parameter Store
		vars, err = m.pullFromParameterStore(ctx, path)
	}

	if err != nil {
		return nil, err
	}

	// Create env file with memory efficiency
	file := env.NewFile()
	
	// Use batch processing for large variable sets
	if len(vars) > 100 {
		return m.pullEnvironmentBatch(ctx, vars)
	}
	
	for key, value := range vars {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		file.Set(key, value)
	}

	return file, nil
}

// ListEnvironmentVariables lists variables for an environment
func (m *Manager) ListEnvironmentVariables(ctx context.Context, envName string) (map[string]string, error) {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		return m.pullFromSecretsManager(ctx, path)
	}

	return m.pullFromParameterStore(ctx, path)
}

// DeleteEnvironment deletes all variables for an environment
func (m *Manager) DeleteEnvironment(ctx context.Context, envName string) error {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		// Delete from Secrets Manager
		secrets, err := m.secretsManager.ListSecrets(ctx, path)
		if err != nil {
			return errors.WrapAWSError(err, "list secrets", path)
		}

		for _, secret := range secrets {
			if err := m.secretsManager.DeleteSecret(ctx, secret.Name, false); err != nil {
				return errors.WrapAWSError(err, "delete secret", secret.Name)
			}
		}
	} else {
		// Delete from Parameter Store
		if err := m.paramStore.DeleteParametersByPath(ctx, path); err != nil {
			return errors.WrapAWSError(err, "delete parameters", path)
		}
	}

	return nil
}

// pushToParameterStore pushes variables to Parameter Store
func (m *Manager) pushToParameterStore(ctx context.Context, path string, vars map[string]string, overwrite bool) error {
	// Ensure path ends with /
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// Check for existing parameters if not forcing overwrite
	if !overwrite {
		existing, err := m.checkExistingParameters(ctx, path, vars)
		if err != nil {
			return err
		}
		
		if len(existing) > 0 {
			// Show existing parameters and ask for action
			action := m.promptBulkOverwrite(existing)
			switch action {
			case "all":
				overwrite = true
			case "none":
				// Remove existing from vars
				for _, key := range existing {
					delete(vars, key)
				}
				if len(vars) == 0 {
					fmt.Println("No new variables to push.")
					return nil
				}
			case "select":
				// Ask for each parameter
				for _, key := range existing {
					if !m.promptOverwriteSingle(key) {
						delete(vars, key)
					}
				}
			case "cancel":
				return fmt.Errorf("push cancelled by user")
			}
		}
	}

	// Use batch processing for large variable sets
	if len(vars) > 50 {
		return m.pushToParameterStoreBatch(ctx, path, vars, overwrite)
	}

	// Push each variable
	for key, value := range vars {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		paramName := path + key
		
		// Determine parameter type based on key
		paramType := "String"
		if strings.Contains(strings.ToLower(key), "password") ||
		   strings.Contains(strings.ToLower(key), "secret") ||
		   strings.Contains(strings.ToLower(key), "key") ||
		   strings.Contains(strings.ToLower(key), "token") {
			paramType = "SecureString"
		}

		err := m.paramStore.PutParameter(ctx, paramName, value, "", paramType, overwrite)
		if err != nil {
			return errors.WrapAWSError(err, "put parameter", paramName)
		}
	}

	return nil
}

// checkExistingParameters checks which parameters already exist
func (m *Manager) checkExistingParameters(ctx context.Context, path string, vars map[string]string) ([]string, error) {
	existing := []string{}
	
	// Get current parameters
	current, err := m.pullFromParameterStore(ctx, path)
	if err != nil {
		// If path doesn't exist, no existing parameters
		return existing, nil
	}
	
	// Check which variables already exist
	for key := range vars {
		if _, exists := current[key]; exists {
			existing = append(existing, key)
		}
	}
	
	return existing, nil
}

// promptBulkOverwrite prompts for bulk overwrite action
func (m *Manager) promptBulkOverwrite(existing []string) string {
	fmt.Printf("\nFound %d existing parameters:\n", len(existing))
	for _, key := range existing {
		fmt.Printf("  - %s\n", key)
	}
	fmt.Println()
	
	// Try interactive menu first
	options := []string{
		"Overwrite all",
		"Skip all (only push new variables)",
		"Select individually",
		"Cancel",
	}
	
	selected, err := prompt.InteractiveSelect("What would you like to do?", options, 1) // Default to "Skip all"
	if err != nil {
		// Fallback to simple menu
		return m.promptBulkOverwriteSimple(existing)
	}
	
	switch selected {
	case 0:
		return "all"
	case 1:
		return "none"
	case 2:
		return "select"
	case 3:
		return "cancel"
	default:
		return "none"
	}
}

// promptBulkOverwriteSimple is a fallback for when interactive mode fails
func (m *Manager) promptBulkOverwriteSimple(existing []string) string {
	options := []prompt.MenuOption{
		{Label: "Overwrite all", Value: "all", Description: ""},
		{Label: "Skip all", Value: "none", Description: "only push new variables"},
		{Label: "Select individually", Value: "select", Description: ""},
		{Label: "Cancel", Value: "cancel", Description: ""},
	}
	
	return prompt.SimpleMenu("\nWhat would you like to do?", options)
}

// promptOverwriteSingle asks the user if they want to overwrite a single parameter
func (m *Manager) promptOverwriteSingle(key string) bool {
	return prompt.InteractiveConfirm(fmt.Sprintf("Overwrite %s?", key), false)
}

// promptOverwriteSecret asks the user if they want to overwrite an existing secret
func (m *Manager) promptOverwriteSecret(secretName string) bool {
	fmt.Printf("\nSecret already exists: %s\n", secretName)
	fmt.Print("Overwrite? [y/N]: ")
	
	var response string
	fmt.Scanln(&response)
	
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// pullFromParameterStore pulls variables from Parameter Store
func (m *Manager) pullFromParameterStore(ctx context.Context, path string) (map[string]string, error) {
	// Get all parameters under the path
	parameters, err := m.paramStore.GetParametersByPath(ctx, path, true, true)
	if err != nil {
		return nil, errors.WrapAWSError(err, "get parameters by path", path)
	}

	// Convert to env vars
	return m.paramStore.ConvertToEnvVars(parameters, path), nil
}

// pushToSecretsManager pushes variables to Secrets Manager
func (m *Manager) pushToSecretsManager(ctx context.Context, path string, vars map[string]string, overwrite bool) error {
	// Clean path for secret name
	secretName := strings.Trim(path, "/")
	secretName = strings.ReplaceAll(secretName, "/", "-")

	// Create or update secret
	err := m.secretsManager.CreateOrUpdateSecret(ctx, secretName, 
		fmt.Sprintf("Environment variables for %s", secretName), vars)
	
	if err != nil {
		if errors.IsAlreadyExistsError(err) && !overwrite {
			// Ask user if they want to overwrite
			if m.promptOverwriteSecret(secretName) {
				// Retry with overwrite
				err = m.secretsManager.CreateOrUpdateSecret(ctx, secretName, 
					fmt.Sprintf("Environment variables for %s", secretName), vars)
				if err != nil {
					return errors.WrapAWSError(err, "create/update secret", secretName)
				}
			} else {
				return fmt.Errorf("secret %s already exists. Update cancelled", secretName)
			}
		} else {
			return errors.WrapAWSError(err, "create/update secret", secretName)
		}
	}

	return nil
}

// pullFromSecretsManager pulls variables from Secrets Manager
func (m *Manager) pullFromSecretsManager(ctx context.Context, path string) (map[string]string, error) {
	// Clean path for secret name
	secretName := strings.Trim(path, "/")
	secretName = strings.ReplaceAll(secretName, "/", "-")

	// Get secret
	secret, err := m.secretsManager.GetSecret(ctx, secretName)
	if err != nil {
		return nil, errors.WrapAWSError(err, "get secret", secretName)
	}

	// Return key-value pairs
	if secret.KeyValue != nil {
		return secret.KeyValue, nil
	}

	// If it's a string value, return as single key
	if secret.Value != "" {
		return map[string]string{
			"SECRET_VALUE": secret.Value,
		}, nil
	}

	return map[string]string{}, nil
}

// GetClient returns the underlying AWS client
func (m *Manager) GetClient() *client.Client {
	return m.client
}

// GetParameterStore returns the Parameter Store client
func (m *Manager) GetParameterStore() *parameter_store.Store {
	return m.paramStore
}

// GetSecretsManager returns the Secrets Manager client
func (m *Manager) GetSecretsManager() *secrets_manager.Manager {
	return m.secretsManager
}

// pullEnvironmentBatch pulls environment variables using batch processing
func (m *Manager) pullEnvironmentBatch(ctx context.Context, vars map[string]string) (*env.File, error) {
	file := env.NewFile()
	batchProcessor := memory.NewBatchProcessor(50, 4) // 50 items per batch, 4 workers
	
	// Create batch jobs
	jobs := make([]memory.BatchJob, 0, len(vars))
	for key, value := range vars {
		key, value := key, value // Capture for closure
		jobs = append(jobs, &setVariableJob{
			file:  file,
			key:   key,
			value: value,
		})
	}
	
	return file, batchProcessor.ProcessBatch(ctx, jobs)
}

// setVariableJob implements BatchJob for setting variables
type setVariableJob struct {
	file  *env.File
	key   string
	value string
}

func (job *setVariableJob) Process() error {
	job.file.Set(job.key, job.value)
	return nil
}

// pushToParameterStoreBatch pushes variables to Parameter Store using batch processing
func (m *Manager) pushToParameterStoreBatch(ctx context.Context, path string, vars map[string]string, overwrite bool) error {
	batchProcessor := memory.NewBatchProcessor(25, 4) // 25 items per batch, 4 workers
	
	// Create batch jobs
	jobs := make([]memory.BatchJob, 0, len(vars))
	for key, value := range vars {
		key, value := key, value // Capture for closure
		jobs = append(jobs, &pushParameterJob{
			manager:   m,
			ctx:       ctx,
			path:      path,
			key:       key,
			value:     value,
			overwrite: overwrite,
		})
	}
	
	return batchProcessor.ProcessBatch(ctx, jobs)
}

// pushParameterJob implements BatchJob for pushing parameters
type pushParameterJob struct {
	manager   *Manager
	ctx       context.Context
	path      string
	key       string
	value     string
	overwrite bool
}

func (job *pushParameterJob) Process() error {
	paramName := job.path + job.key
	
	// Determine parameter type based on key
	paramType := "String"
	if strings.Contains(strings.ToLower(job.key), "password") ||
	   strings.Contains(strings.ToLower(job.key), "secret") ||
	   strings.Contains(strings.ToLower(job.key), "key") ||
	   strings.Contains(strings.ToLower(job.key), "token") {
		paramType = "SecureString"
	}

	err := job.manager.paramStore.PutParameter(job.ctx, paramName, job.value, "", paramType, job.overwrite)
	if err != nil {
		// Check if it's an already exists error and overwrite is false
		if errors.IsAlreadyExistsError(err) && !job.overwrite {
			return fmt.Errorf("parameter %s already exists. Use --force to overwrite", paramName)
		}
		return errors.WrapAWSError(err, "put parameter", paramName)
	}
	
	return nil
}

// PushEnvironmentWithMemoryOptimization pushes environment variables with memory optimization
func (m *Manager) PushEnvironmentWithMemoryOptimization(ctx context.Context, envName string, file *env.File, overwrite bool) error {
	// Monitor memory usage during operation
	poolManager := memory.GetGlobalPoolManager()
	if poolManager != nil {
		defer func() {
			// Force GC after operation
			poolManager.ForceGC()
		}()
	}
	
	return m.PushEnvironment(ctx, envName, file, overwrite)
}

// PullEnvironmentWithStreaming pulls environment variables using streaming approach
func (m *Manager) PullEnvironmentWithStreaming(ctx context.Context, envName string, writer func(*env.Variable) error) error {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	var vars map[string]string

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		// Pull from Secrets Manager
		vars, err = m.pullFromSecretsManager(ctx, path)
	} else {
		// Pull from Parameter Store
		vars, err = m.pullFromParameterStore(ctx, path)
	}

	if err != nil {
		return err
	}

	// Stream variables to writer
	lineNum := 1
	for key, value := range vars {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		variable := &env.Variable{
			Key:   key,
			Value: value,
			Line:  lineNum,
		}
		
		if err := writer(variable); err != nil {
			return fmt.Errorf("writer error for variable %s: %w", key, err)
		}
		
		lineNum++
	}

	return nil
}