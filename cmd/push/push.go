package push

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/color"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/parallel"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	environment    string
	prefix         string
	variables      string
	force          bool
	dryRun         bool
	all            bool
	showDiff       bool
	parallelMode   bool
	maxWorkers     int
	batchSize      int
	skipEmpty      bool
	allowDuplicate bool
	noProgress     bool
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push environment variables to AWS",
	Long: `Push environment variables to AWS Parameter Store or Secrets Manager.

This command reads your local .env files and uploads the variables to AWS
based on your configuration in .envyrc.`,
	Example: `  # Push variables for the default environment
  envy push
  
  # Push variables for a specific environment
  envy push --env staging
  
  # Push with custom prefix
  envy push --prefix "/myapp/prod/"
  
  # Push specific variables only
  envy push --vars "API_KEY,DATABASE_URL"
  
  # Force overwrite existing parameters
  envy push --force
  
  # Dry run to see what would be pushed
  envy push --dry-run`,
	RunE: runPush,
}

func init() {
	root.GetRootCmd().AddCommand(pushCmd)
	
	// Add flags specific to push command
	pushCmd.Flags().StringVarP(&environment, "env", "e", "", "Target environment")
	pushCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "AWS parameter prefix (overrides config)")
	pushCmd.Flags().StringVarP(&variables, "vars", "v", "", "Comma-separated list of variables to push")
	pushCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite existing parameters")
	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pushed without making changes")
	pushCmd.Flags().BoolVarP(&all, "all", "a", false, "Push all environments")
	pushCmd.Flags().BoolVar(&showDiff, "diff", false, "Show differences before pushing")
	pushCmd.Flags().BoolVar(&parallelMode, "parallel", false, "Enable parallel upload")
	pushCmd.Flags().IntVar(&maxWorkers, "max-workers", 10, "Maximum number of parallel workers")
	pushCmd.Flags().IntVar(&batchSize, "batch-size", 10, "Batch size for parallel operations")
	pushCmd.Flags().BoolVar(&skipEmpty, "skip-empty", true, "Skip variables with empty values")
	pushCmd.Flags().BoolVar(&allowDuplicate, "allow-duplicate", false, "Allow duplicate variable names (use last value)")
	pushCmd.Flags().BoolVar(&noProgress, "no-progress", false, "Disable progress bar")
}

func runPush(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create AWS manager
	awsManager, err := aws.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create AWS manager: %w", err)
	}

	// Determine which environments to push
	environments := []string{}
	if all {
		for envName := range cfg.Environments {
			environments = append(environments, envName)
		}
	} else {
		if environment == "" {
			environment = cfg.DefaultEnvironment
		}
		environments = []string{environment}
	}

	// Process each environment
	for _, envName := range environments {
		if err := pushEnvironment(ctx, cfg, awsManager, envName); err != nil {
			return fmt.Errorf("failed to push environment %s: %w", envName, err)
		}
	}

	return nil
}

func pushEnvironment(ctx context.Context, cfg *config.Config, awsManager *aws.Manager, envName string) error {
	color.PrintInfo("Pushing environment: %s", envName)

	// Get environment configuration
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Create environment manager
	envManager := env.NewManager(".")

	// Load and merge environment files
	envFile, err := envManager.LoadFiles(envConfig.Files)
	if err != nil {
		return fmt.Errorf("failed to load environment files: %w", err)
	}

	// Filter variables if specified
	if variables != "" {
		varsToKeep := strings.Split(variables, ",")
		filteredFile := env.NewFile()
		
		for _, varName := range varsToKeep {
			varName = strings.TrimSpace(varName)
			if value, exists := envFile.Get(varName); exists {
				filteredFile.Set(varName, value)
			} else {
				color.PrintWarning("Variable %s not found in local files", varName)
			}
		}
		
		envFile = filteredFile
	}

	// Check for duplicate keys
	duplicates := checkDuplicates(envFile)
	if len(duplicates) > 0 && !allowDuplicate {
		color.PrintWarning("Duplicate variables found: %v", duplicates)
		fmt.Println("Use --allow-duplicate to use the last value for duplicates")
		return fmt.Errorf("duplicate variables found")
	}

	// Filter out empty values if requested
	if skipEmpty {
		filteredFile := env.NewFile()
		for _, key := range envFile.SortedKeys() {
			value, _ := envFile.Get(key)
			if value != "" {
				filteredFile.Set(key, value)
			}
		}
		envFile = filteredFile
	}

	// Show what will be pushed
	color.PrintBold("\nVariables to push:")
	skippedEmpty := 0
	for _, key := range envFile.SortedKeys() {
		value, _ := envFile.Get(key)
		displayValue := value
		if isSensitive(key) {
			displayValue = "***HIDDEN***"
		}
		if value == "" && skipEmpty {
			skippedEmpty++
			continue
		}
		fmt.Printf("  %s = %s\n", key, displayValue)
	}
	if skippedEmpty > 0 {
		color.PrintInfo("\n(%d empty variables will be skipped)", skippedEmpty)
	}

	// Get current remote variables if showing diff
	if showDiff && !dryRun {
		color.PrintInfo("\nFetching current remote values...")
		remoteVars, err := awsManager.ListEnvironmentVariables(ctx, envName)
		if err != nil {
			color.PrintWarning("Could not fetch remote values: %v", err)
		} else {
			showDifferences(envFile.ToMap(), remoteVars)
		}
	}

	// Dry run mode
	if dryRun {
		color.PrintWarning("\n[DRY RUN] No changes will be made")
		color.PrintInfo("Would push %d variables to %s", len(envFile.Keys()), getTargetDescription(cfg, envName))
		return nil
	}

	// Confirmation prompt if not forced
	if !force && !confirmPush(len(envFile.Keys()), envName) {
		color.PrintWarning("Push cancelled")
		return nil
	}

	// Push to AWS
	color.PrintInfo("\nPushing to %s...", getTargetDescription(cfg, envName))
	
	if parallelMode {
		// Use parallel push
		if err := pushParallel(ctx, awsManager, envName, envFile, force); err != nil {
			return fmt.Errorf("parallel push failed: %w", err)
		}
	} else {
		// Use sequential push with progress
		if err := pushWithProgress(ctx, awsManager, envName, envFile, force); err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
	}

	color.PrintSuccess("Successfully pushed %d variables to %s", len(envFile.Keys()), envName)
	return nil
}

func showDifferences(local, remote map[string]string) {
	color.PrintBold("\nDifferences:")
	
	// Find added variables
	added := []string{}
	for key := range local {
		if _, exists := remote[key]; !exists {
			added = append(added, key)
		}
	}
	
	if len(added) > 0 {
		color.PrintInfo("  Added:")
		for _, key := range added {
			fmt.Printf("    %s %s\n", color.FormatSuccess("+"), key)
		}
	}

	// Find modified variables
	modified := []string{}
	for key, localValue := range local {
		if remoteValue, exists := remote[key]; exists && localValue != remoteValue {
			modified = append(modified, key)
		}
	}
	
	if len(modified) > 0 {
		color.PrintInfo("  Modified:")
		for _, key := range modified {
			fmt.Printf("    %s %s\n", color.FormatWarning("~"), key)
		}
	}

	// Find removed variables
	removed := []string{}
	for key := range remote {
		if _, exists := local[key]; !exists {
			removed = append(removed, key)
		}
	}
	
	if len(removed) > 0 {
		color.PrintInfo("  Will remain in remote (not in local):")
		for _, key := range removed {
			fmt.Printf("    %s %s\n", color.FormatInfo("?"), key)
		}
	}

	if len(added) == 0 && len(modified) == 0 {
		color.PrintInfo("  No changes detected")
	}
}

func confirmPush(count int, envName string) bool {
	fmt.Printf("\n%s Continue? [y/N]: ", color.FormatWarning(fmt.Sprintf("About to push %d variables to %s.", count, envName)))
	
	var response string
	fmt.Scanln(&response)
	
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}


func getTargetDescription(cfg *config.Config, envName string) string {
	service := cfg.GetAWSService(envName)
	path := cfg.GetParameterPath(envName)
	region := cfg.AWS.Region
	
	if service == "secrets_manager" {
		return fmt.Sprintf("AWS Secrets Manager (%s)", region)
	}
	
	return fmt.Sprintf("AWS Parameter Store %s (%s)", path, region)
}

// pushParallel pushes environment variables in parallel
func pushParallel(ctx context.Context, awsManager *aws.Manager, envName string, envFile *env.File, overwrite bool) error {
	log.Info("Starting parallel upload",
		zap.String("environment", envName),
		zap.Int("variables", len(envFile.Keys())),
		zap.Int("max_workers", maxWorkers),
		zap.Int("batch_size", batchSize),
	)

	// Get environment configuration
	cfg := awsManager.GetConfig()
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Determine which service to use
	service := cfg.GetAWSService(envName)
	path := cfg.GetParameterPath(envName)

	// Create parallel manager
	parallelManager := aws.NewParallelManager(awsManager, aws.ParallelOptions{
		MaxWorkers: maxWorkers,
		BatchSize:  batchSize,
		Timeout:    30 * time.Second,
	})

	// Create tasks for each variable
	var tasks []parallel.Task
	for _, key := range envFile.SortedKeys() {
		value, _ := envFile.Get(key)
		
		// Create task based on service type
		if service == "secrets_manager" || envConfig.UseSecretsManager {
			task := createSecretsManagerTask(key, value, path, overwrite, parallelManager)
			tasks = append(tasks, task)
		} else {
			task := createParameterStoreTask(key, value, path, overwrite, parallelManager)
			tasks = append(tasks, task)
		}
	}

	// Create batch processor with progress
	processor := parallel.NewBatchProgressProcessor(ctx, parallelManager.GetMaxWorkers(), true)
	
	// Convert tasks to interface slice
	items := make([]interface{}, len(tasks))
	for i, task := range tasks {
		items[i] = task
	}
	
	// Process tasks with progress
	results, err := processor.ProcessWithProgress(
		ctx,
		items,
		"Uploading environment variables",
		func(ctx context.Context, item interface{}) error {
			task := item.(parallel.Task)
			return task.Execute(ctx)
		},
	)

	if err != nil {
		return err
	}

	// Check for errors
	var failedVars []string
	for _, result := range results {
		if result.Error != nil {
			failedVars = append(failedVars, result.Task.Name())
			log.Error("Failed to upload variable",
				zap.String("variable", result.Task.Name()),
				zap.Error(result.Error),
			)
		}
	}

	if len(failedVars) > 0 {
		return errors.New(errors.ErrAWSConnection, fmt.Sprintf("Failed to upload %d variables: %v", len(failedVars), failedVars))
	}

	return nil
}

// createParameterStoreTask creates a task for Parameter Store upload
func createParameterStoreTask(key, value, path string, overwrite bool, manager *aws.ParallelManager) parallel.Task {
	paramName := path
	if !strings.HasSuffix(paramName, "/") {
		paramName = paramName + "/"
	}
	paramName = paramName + key

	// Determine parameter type
	paramType := "String"
	if isSensitive(key) {
		paramType = "SecureString"
	}

	return parallel.NewTaskFunc(
		key,
		func(ctx context.Context) error {
			return manager.PutParameter(ctx, paramName, value, paramType, overwrite)
		},
		true, // Retriable
	)
}

// createSecretsManagerTask creates a task for Secrets Manager upload
func createSecretsManagerTask(key, value, path string, overwrite bool, manager *aws.ParallelManager) parallel.Task {
	// For Secrets Manager, we typically batch all variables into one secret
	// This is a simplified version for individual variables
	secretName := strings.Trim(path, "/")
	secretName = strings.ReplaceAll(secretName, "/", "-") + "-" + key

	return parallel.NewTaskFunc(
		key,
		func(ctx context.Context) error {
			return manager.PutSecret(ctx, secretName, map[string]string{key: value}, overwrite)
		},
		true, // Retriable
	)
}

// isSensitive checks if the key represents a sensitive value
func isSensitive(key string) bool {
	// Exact matches (case-insensitive)
	exactMatches := []string{
		"password", "passwd", "pwd", "secret", "token", 
		"api_key", "apikey", "access_key", "accesskey",
		"private_key", "privatekey", "auth_token", "authtoken",
	}
	
	// Suffix patterns (must end with these)
	suffixPatterns := []string{
		"_password", "_passwd", "_pwd", "_secret", "_token",
		"_key", "_auth", "_credential", "_private",
	}
	
	// Prefix patterns (must start with these)
	prefixPatterns := []string{
		"secret_", "private_", "auth_",
	}
	
	keyLower := strings.ToLower(key)
	
	// Check exact matches
	for _, pattern := range exactMatches {
		if keyLower == pattern {
			return true
		}
	}
	
	// Check suffix patterns
	for _, pattern := range suffixPatterns {
		if strings.HasSuffix(keyLower, pattern) {
			return true
		}
	}
	
	// Check prefix patterns
	for _, pattern := range prefixPatterns {
		if strings.HasPrefix(keyLower, pattern) {
			return true
		}
	}
	
	return false
}

// pushWithProgress pushes environment variables with a progress bar
func pushWithProgress(ctx context.Context, awsManager *aws.Manager, envName string, envFile *env.File, overwrite bool) error {
	cfg := awsManager.GetConfig()
	service := cfg.GetAWSService(envName)
	
	// For Secrets Manager, use regular push (single operation)
	if service == "secrets_manager" {
		return awsManager.PushEnvironment(ctx, envName, envFile, overwrite)
	}
	
	// For Parameter Store, show progress for each variable
	vars := envFile.ToMap()
	
	// Check if progress should be shown
	showProgress := !viper.GetBool("quiet") && !noProgress
	
	if !showProgress || len(vars) == 0 {
		// Use regular push without progress
		return awsManager.PushEnvironment(ctx, envName, envFile, overwrite)
	}
	
	// Create progress bar
	bar := progressbar.NewOptions(len(vars),
		progressbar.OptionSetDescription("Pushing variables to AWS"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("vars"),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]█[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)
	
	// Get path
	path := cfg.GetParameterPath(envName)
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	
	// Check for existing parameters if not forcing overwrite
	existingVars := make(map[string]bool)
	if !overwrite {
		// Let the manager handle the interactive overwrite prompts
		// Use the regular push method which includes the interactive prompts
		return awsManager.PushEnvironment(ctx, envName, envFile, overwrite)
	}
	
	// Push each variable with progress update
	failedVars := []string{}
	paramStore := awsManager.GetParameterStore()
	
	for key, value := range vars {
		// Check if should skip existing
		if !overwrite && existingVars[key] {
			bar.Add(1)
			continue
		}
		
		paramName := path + key
		
		// Determine parameter type
		paramType := "String"
		if isSensitive(key) {
			paramType = "SecureString"
		}
		
		// Push parameter
		err := paramStore.PutParameter(ctx, paramName, value, "", paramType, overwrite)
		if err != nil {
			failedVars = append(failedVars, key)
		}
		
		bar.Add(1)
	}
	
	bar.Finish()
	
	// Report any failures
	if len(failedVars) > 0 {
		return fmt.Errorf("failed to push %d variables: %v", len(failedVars), failedVars)
	}
	
	return nil
}

// checkDuplicates returns a list of duplicate variable names
func checkDuplicates(file *env.File) []string {
	seen := make(map[string]int)
	duplicates := []string{}
	
	for _, key := range file.Keys() {
		seen[key]++
	}
	
	for key, count := range seen {
		if count > 1 {
			duplicates = append(duplicates, key)
		}
	}
	
	return duplicates
}