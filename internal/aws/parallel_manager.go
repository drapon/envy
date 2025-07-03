package aws

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/drapon/envy/internal/aws/errors"
	parameter_store "github.com/drapon/envy/internal/aws/parameter_store"
	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/parallel"
	"go.uber.org/zap"
)

// ParallelManager is an AWS manager with parallel processing capabilities
type ParallelManager struct {
	*Manager
	maxWorkers int
	batchSize  int
	rateLimit  int
}

// GetMaxWorkers returns the maximum number of workers
func (pm *ParallelManager) GetMaxWorkers() int {
	return pm.maxWorkers
}

// ParallelOptions contains options for parallel processing
type ParallelOptions struct {
	MaxWorkers int
	BatchSize  int
	RateLimit  int // requests per second
	Timeout    time.Duration
}

// DefaultParallelOptions returns default parallel options
func DefaultParallelOptions() ParallelOptions {
	return ParallelOptions{
		MaxWorkers: runtime.NumCPU(),
		BatchSize:  100,
		RateLimit:  0, // No limit by default
	}
}

// NewParallelManager creates a new parallel AWS manager
func NewParallelManager(manager *Manager, opts ParallelOptions) *ParallelManager {
	// Apply defaults
	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = runtime.NumCPU()
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}

	return &ParallelManager{
		Manager:    manager,
		maxWorkers: opts.MaxWorkers,
		batchSize:  opts.BatchSize,
		rateLimit:  opts.RateLimit,
	}
}

// PushEnvironmentParallel pushes environment variables to AWS in parallel
func (m *ParallelManager) PushEnvironmentParallel(
	ctx context.Context,
	envName string,
	file *env.File,
	overwrite bool,
	showProgress bool,
) error {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	// Convert to map
	vars := file.ToMap()

	log.Info("並列プッシュ開始",
		zap.String("environment", envName),
		zap.String("service", service),
		zap.Int("variables", len(vars)),
		zap.Int("max_workers", m.maxWorkers),
	)

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		// Secrets Manager doesn't benefit from parallel push (single secret)
		return m.pushToSecretsManager(ctx, path, vars, overwrite)
	}

	// Use parallel processing for Parameter Store
	return m.pushToParameterStoreParallel(ctx, path, vars, overwrite, showProgress)
}

// pushToParameterStoreParallel pushes variables to Parameter Store in parallel
func (m *ParallelManager) pushToParameterStoreParallel(
	ctx context.Context,
	path string,
	vars map[string]string,
	overwrite bool,
	showProgress bool,
) error {
	// Ensure path ends with /
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// Create AWS batch processor
	processor := parallel.NewAWSBatchProcessor(
		ctx,
		"parameter_store",
		m.maxWorkers,
		parallel.WithBatchSize(m.batchSize),
	)

	// Convert variables to operations
	type paramOp struct {
		Key   string
		Value string
		Path  string
	}

	var operations []interface{}
	for key, value := range vars {
		operations = append(operations, &paramOp{
			Key:   key,
			Value: value,
			Path:  path + key,
		})
	}

	// Create progress processor if needed
	var results []parallel.Result
	var err error

	if showProgress {
		progressProcessor := parallel.NewBatchProgressProcessor(
			ctx,
			m.maxWorkers,
			true,
			parallel.WithBatchSize(m.batchSize),
		)

		results, err = progressProcessor.ProcessWithProgress(
			ctx,
			operations,
			"環境変数をParameter Storeにアップロード中",
			func(ctx context.Context, item interface{}) error {
				op := item.(*paramOp)

				// Determine parameter type
				paramType := "String"
				if isSensitiveKey(op.Key) {
					paramType = "SecureString"
				}

				// Push parameter
				return m.paramStore.PutParameter(ctx, op.Path, op.Value, "", paramType, overwrite)
			},
		)
	} else {
		results, err = processor.ProcessAWSOperations(
			ctx,
			operations,
			func(ctx context.Context, item interface{}) error {
				op := item.(*paramOp)

				// Determine parameter type
				paramType := "String"
				if isSensitiveKey(op.Key) {
					paramType = "SecureString"
				}

				// Push parameter
				return m.paramStore.PutParameter(ctx, op.Path, op.Value, "", paramType, overwrite)
			},
		)
	}

	if err != nil {
		return fmt.Errorf("並列処理エラー: %w", err)
	}

	// Check for individual errors
	var errorCount int
	errorDetails := make(map[string]error)

	for _, result := range results {
		if result.Error != nil {
			errorCount++
			op := result.Task.(*parallel.TaskFunc).Name()
			errorDetails[op] = result.Error

			// Check if it's an already exists error
			if errors.IsAlreadyExistsError(result.Error) && !overwrite {
				return fmt.Errorf("パラメータが既に存在します。--forceオプションで上書きできます")
			}
		}
	}

	if errorCount > 0 {
		log.Error("一部のパラメータのプッシュに失敗",
			zap.Int("failed", errorCount),
			zap.Int("total", len(operations)),
		)

		// Return first error for now
		for _, err := range errorDetails {
			return err
		}
	}

	log.Info("並列プッシュ完了",
		zap.Int("succeeded", len(operations)-errorCount),
		zap.Int("failed", errorCount),
	)

	return nil
}

// PullEnvironmentParallel pulls environment variables from AWS in parallel
func (m *ParallelManager) PullEnvironmentParallel(
	ctx context.Context,
	envName string,
	showProgress bool,
) (*env.File, error) {
	// Get environment configuration
	envConfig, err := m.config.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	// Determine which service to use
	service := m.config.GetAWSService(envName)
	path := m.config.GetParameterPath(envName)

	log.Info("並列プル開始",
		zap.String("environment", envName),
		zap.String("service", service),
	)

	var vars map[string]string

	if service == "secrets_manager" || envConfig.UseSecretsManager {
		// Secrets Manager doesn't benefit from parallel pull (single secret)
		vars, err = m.pullFromSecretsManager(ctx, path)
	} else {
		// Use parallel processing for Parameter Store
		vars, err = m.pullFromParameterStoreParallel(ctx, path, showProgress)
	}

	if err != nil {
		return nil, err
	}

	// Create env file
	file := env.NewFile()
	for key, value := range vars {
		file.Set(key, value)
	}

	return file, nil
}

// pullFromParameterStoreParallel pulls variables from Parameter Store in parallel
func (m *ParallelManager) pullFromParameterStoreParallel(
	ctx context.Context,
	path string,
	showProgress bool,
) (map[string]string, error) {
	// First, list all parameters under the path
	parameters, err := m.paramStore.GetParametersByPath(ctx, path, false, false)
	if err != nil {
		return nil, errors.WrapAWSError(err, "list parameters", path)
	}

	if len(parameters) == 0 {
		return map[string]string{}, nil
	}

	// Create batch processor
	processor := parallel.NewAWSBatchProcessor(
		ctx,
		"parameter_store",
		m.maxWorkers,
		parallel.WithBatchSize(m.batchSize),
	)

	// Convert parameters to operations
	var operations []interface{}
	for _, param := range parameters {
		operations = append(operations, param)
	}

	// Result storage
	results := make(map[string]string)
	var mu sync.Mutex

	// Process function
	processFn := func(ctx context.Context, item interface{}) error {
		param := item.(*parameter_store.Parameter)

		// Get parameter value with decryption
		fullParam, err := m.paramStore.GetParameter(ctx, param.Name, true)
		if err != nil {
			return errors.WrapAWSError(err, "get parameter", param.Name)
		}

		// Extract key from path
		key := strings.TrimPrefix(fullParam.Name, path)
		key = strings.TrimPrefix(key, "/")

		mu.Lock()
		results[key] = fullParam.Value
		mu.Unlock()

		return nil
	}

	// Process with or without progress
	var processResults []parallel.Result
	if showProgress {
		progressProcessor := parallel.NewBatchProgressProcessor(
			ctx,
			m.maxWorkers,
			true,
			parallel.WithBatchSize(m.batchSize),
		)

		processResults, err = progressProcessor.ProcessWithProgress(
			ctx,
			operations,
			"Parameter Storeから環境変数をダウンロード中",
			processFn,
		)
	} else {
		processResults, err = processor.ProcessAWSOperations(ctx, operations, processFn)
	}

	if err != nil {
		return nil, fmt.Errorf("並列処理エラー: %w", err)
	}

	// Check for errors
	var errorCount int
	for _, result := range processResults {
		if result.Error != nil {
			errorCount++
			log.Error("パラメータ取得エラー",
				zap.String("parameter", result.Task.Name()),
				zap.Error(result.Error),
			)
		}
	}

	if errorCount > 0 {
		log.Warn("一部のパラメータの取得に失敗",
			zap.Int("failed", errorCount),
			zap.Int("total", len(operations)),
		)
	}

	return results, nil
}

// ListEnvironmentsParallel lists variables for multiple environments in parallel
func (m *ParallelManager) ListEnvironmentsParallel(
	ctx context.Context,
	envNames []string,
	showProgress bool,
) (map[string]map[string]string, error) {
	if len(envNames) == 0 {
		return map[string]map[string]string{}, nil
	}

	// Create processor
	processor := parallel.NewBatchProgressProcessor(
		ctx,
		m.maxWorkers,
		showProgress,
		parallel.WithBatchSize(1), // Each environment is a batch
	)

	// Convert environments to operations
	var operations []interface{}
	for _, envName := range envNames {
		operations = append(operations, envName)
	}

	// Result storage
	results := make(map[string]map[string]string)
	var mu sync.Mutex

	// Process environments
	processResults, err := processor.ProcessWithProgress(
		ctx,
		operations,
		"環境情報を取得中",
		func(ctx context.Context, item interface{}) error {
			envName := item.(string)

			// Get variables for this environment
			vars, err := m.ListEnvironmentVariables(ctx, envName)
			if err != nil {
				return err
			}

			mu.Lock()
			results[envName] = vars
			mu.Unlock()

			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	// Check for errors
	var errorCount int
	for _, result := range processResults {
		if result.Error != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		return results, fmt.Errorf("%d個の環境の取得に失敗しました", errorCount)
	}

	return results, nil
}

// ValidateEnvironmentsParallel validates multiple environments in parallel
func (m *ParallelManager) ValidateEnvironmentsParallel(
	ctx context.Context,
	envNames []string,
	validator func(envName string, vars map[string]string) error,
	showProgress bool,
) (map[string]error, error) {
	if len(envNames) == 0 {
		return map[string]error{}, nil
	}

	// First, get all environment variables in parallel
	envVars, err := m.ListEnvironmentsParallel(ctx, envNames, false)
	if err != nil {
		return nil, err
	}

	// Create processor for validation
	processor := parallel.NewBatchProgressProcessor(
		ctx,
		m.maxWorkers,
		showProgress,
		parallel.WithBatchSize(1),
	)

	// Convert to operations
	type validationOp struct {
		EnvName string
		Vars    map[string]string
	}

	var operations []interface{}
	for envName, vars := range envVars {
		operations = append(operations, &validationOp{
			EnvName: envName,
			Vars:    vars,
		})
	}

	// Result storage
	validationErrors := make(map[string]error)
	var mu sync.Mutex

	// Process validations
	processResults, err := processor.ProcessWithProgress(
		ctx,
		operations,
		"環境変数を検証中",
		func(ctx context.Context, item interface{}) error {
			op := item.(*validationOp)

			// Run validation
			if err := validator(op.EnvName, op.Vars); err != nil {
				mu.Lock()
				validationErrors[op.EnvName] = err
				mu.Unlock()
				return err
			}

			return nil
		},
	)

	if err != nil {
		return validationErrors, err
	}

	// Check results
	for _, result := range processResults {
		if result.Error != nil {
			log.Warn("環境検証エラー",
				zap.String("environment", result.Task.Name()),
				zap.Error(result.Error),
			)
		}
	}

	return validationErrors, nil
}

// isSensitiveKey checks if a key contains sensitive information
func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "secret", "key", "token",
		"credential", "auth", "private", "cert",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}

	return false
}

// PutParameter puts a single parameter to Parameter Store
func (m *ParallelManager) PutParameter(ctx context.Context, name, value, paramType string, overwrite bool) error {
	return m.paramStore.PutParameter(ctx, name, value, "", paramType, overwrite)
}

// PutSecret puts a secret to Secrets Manager
func (m *ParallelManager) PutSecret(ctx context.Context, name string, data map[string]string, overwrite bool) error {
	return m.secretsManager.CreateOrUpdateSecret(ctx, name,
		fmt.Sprintf("Environment variables for %s", name), data)
}
