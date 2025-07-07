package diff

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	from        string
	to          string
	file1       string
	file2       string
	format      string
	changes     string
	environment string
	showValues  bool
	colorOutput bool
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between environments",
	Long: `Show differences between local and remote environment variables,
or between different environments or files.`,
	Example: `  # Compare local file with AWS
  envy diff
  
  # Compare two environments
  envy diff --from dev --to prod
  
  # Compare two files
  envy diff --file1 .env.dev --file2 .env.prod
  
  # Show only additions
  envy diff --changes additions
  
  # Output as JSON
  envy diff --format json`,
	RunE: runDiff,
}

// GetDiffCmd returns the diff command.
func GetDiffCmd() *cobra.Command {
	return diffCmd
}

func init() {
	root.GetRootCmd().AddCommand(diffCmd)

	// Add flags specific to diff command
	diffCmd.Flags().StringVar(&from, "from", "local", "Source environment or 'local'")
	diffCmd.Flags().StringVar(&to, "to", "aws", "Target environment or 'aws'")
	diffCmd.Flags().StringVar(&file1, "file1", "", "First file to compare")
	diffCmd.Flags().StringVar(&file2, "file2", "", "Second file to compare")
	diffCmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	diffCmd.Flags().StringVarP(&changes, "changes", "c", "all", "Show changes (all/additions/deletions/modifications)")
	diffCmd.Flags().StringVarP(&environment, "env", "e", "", "Environment to use for comparison")
	diffCmd.Flags().BoolVar(&showValues, "show-values", false, "Show actual values in diff")
	diffCmd.Flags().BoolVar(&colorOutput, "color", true, "Enable colored output")
}

func runDiff(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// If files are specified, compare them directly
	if file1 != "" && file2 != "" {
		return compareFiles(file1, file2)
	}

	// Load configuration
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Use default environment if not specified
	if environment == "" {
		environment = cfg.DefaultEnvironment
	}

	// Get variables for comparison
	var vars1, vars2 map[string]string
	var source1, source2 string

	// Handle 'from' source
	if from == "local" {
		vars1, err = getLocalVariables(cfg, environment)
		source1 = "local files"
	} else if from == "aws" {
		vars1, err = getAWSVariables(ctx, cfg, environment)
		source1 = "AWS"
	} else {
		// Treat as environment name
		if from == environment {
			vars1, err = getLocalVariables(cfg, from)
			source1 = fmt.Sprintf("local %s", from)
		} else {
			vars1, err = getAWSVariables(ctx, cfg, from)
			source1 = fmt.Sprintf("AWS %s", from)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get variables from %s: %w", source1, err)
	}

	// Handle 'to' source
	if to == "local" {
		vars2, err = getLocalVariables(cfg, environment)
		source2 = "local files"
	} else if to == "aws" {
		vars2, err = getAWSVariables(ctx, cfg, environment)
		source2 = "AWS"
	} else {
		// Treat as environment name
		if to == environment {
			vars2, err = getLocalVariables(cfg, to)
			source2 = fmt.Sprintf("local %s", to)
		} else {
			vars2, err = getAWSVariables(ctx, cfg, to)
			source2 = fmt.Sprintf("AWS %s", to)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get variables from %s: %w", source2, err)
	}

	// Calculate differences
	diff := calculateDiff(vars1, vars2)

	// Display results
	if format == "json" {
		return displayJSONDiff(diff, source1, source2)
	}

	return displayTextDiff(diff, source1, source2)
}

func compareFiles(file1Path, file2Path string) error {
	// Load first file
	f1, err := env.ParseFile(file1Path)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", file1Path, err)
	}

	// Load second file
	f2, err := env.ParseFile(file2Path)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", file2Path, err)
	}

	// Calculate differences
	diff := calculateDiff(f1.ToMap(), f2.ToMap())

	// Display results
	if format == "json" {
		return displayJSONDiff(diff, file1Path, file2Path)
	}

	return displayTextDiff(diff, file1Path, file2Path)
}

func getLocalVariables(cfg *config.Config, envName string) (map[string]string, error) {
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	manager := env.NewManager(".")
	file, err := manager.LoadFiles(envConfig.Files)
	if err != nil {
		return nil, err
	}

	return file.ToMap(), nil
}

func getAWSVariables(ctx context.Context, cfg *config.Config, envName string) (map[string]string, error) {
	awsManager, err := aws.NewManager(cfg)
	if err != nil {
		return nil, err
	}

	return awsManager.ListEnvironmentVariables(ctx, envName)
}

type DiffResult struct {
	Added     map[string]string
	Deleted   map[string]string
	Modified  map[string][2]string // [old, new]
	Unchanged map[string]string
}

func calculateDiff(from, to map[string]string) *DiffResult {
	result := &DiffResult{
		Added:     make(map[string]string),
		Deleted:   make(map[string]string),
		Modified:  make(map[string][2]string),
		Unchanged: make(map[string]string),
	}

	// Check all keys in 'from'
	for key, fromValue := range from {
		if toValue, exists := to[key]; exists {
			if fromValue != toValue {
				result.Modified[key] = [2]string{fromValue, toValue}
			} else {
				result.Unchanged[key] = fromValue
			}
		} else {
			result.Deleted[key] = fromValue
		}
	}

	// Check for added keys in 'to'
	for key, toValue := range to {
		if _, exists := from[key]; !exists {
			result.Added[key] = toValue
		}
	}

	return result
}

func displayTextDiff(diff *DiffResult, source1, source2 string) error {
	fmt.Printf("Comparing %s → %s\n\n", source1, source2)

	hasChanges := false

	// Show additions
	if (changes == "all" || changes == "additions") && len(diff.Added) > 0 {
		hasChanges = true
		if colorOutput {
			fmt.Print("\033[32m") // Green
		}
		fmt.Println("Added:")
		keys := sortedKeys(diff.Added)
		for _, key := range keys {
			if showValues {
				fmt.Printf("  + %s = %s\n", key, diff.Added[key])
			} else {
				fmt.Printf("  + %s\n", key)
			}
		}
		if colorOutput {
			fmt.Print("\033[0m") // Reset
		}
		fmt.Println()
	}

	// Show deletions
	if (changes == "all" || changes == "deletions") && len(diff.Deleted) > 0 {
		hasChanges = true
		if colorOutput {
			fmt.Print("\033[31m") // Red
		}
		fmt.Println("Deleted:")
		keys := sortedKeys(diff.Deleted)
		for _, key := range keys {
			if showValues {
				fmt.Printf("  - %s = %s\n", key, diff.Deleted[key])
			} else {
				fmt.Printf("  - %s\n", key)
			}
		}
		if colorOutput {
			fmt.Print("\033[0m") // Reset
		}
		fmt.Println()
	}

	// Show modifications
	if (changes == "all" || changes == "modifications") && len(diff.Modified) > 0 {
		hasChanges = true
		if colorOutput {
			fmt.Print("\033[33m") // Yellow
		}
		fmt.Println("Modified:")
		keys := sortedKeysModified(diff.Modified)
		for _, key := range keys {
			values := diff.Modified[key]
			if showValues {
				fmt.Printf("  ~ %s\n", key)
				fmt.Printf("    - %s\n", maskValue(key, values[0]))
				fmt.Printf("    + %s\n", maskValue(key, values[1]))
			} else {
				fmt.Printf("  ~ %s\n", key)
			}
		}
		if colorOutput {
			fmt.Print("\033[0m") // Reset
		}
		fmt.Println()
	}

	if !hasChanges {
		fmt.Println("No differences found")
	} else {
		// Summary
		fmt.Printf("Summary: %d added, %d deleted, %d modified, %d unchanged\n",
			len(diff.Added), len(diff.Deleted), len(diff.Modified), len(diff.Unchanged))
	}

	return nil
}

func displayJSONDiff(diff *DiffResult, source1, source2 string) error {
	// Simple JSON output
	fmt.Println("{")
	fmt.Printf("  \"from\": \"%s\",\n", source1)
	fmt.Printf("  \"to\": \"%s\",\n", source2)
	fmt.Printf("  \"added\": %d,\n", len(diff.Added))
	fmt.Printf("  \"deleted\": %d,\n", len(diff.Deleted))
	fmt.Printf("  \"modified\": %d,\n", len(diff.Modified))
	fmt.Printf("  \"unchanged\": %d,\n", len(diff.Unchanged))

	if showValues {
		fmt.Println("  \"changes\": {")

		if len(diff.Added) > 0 {
			fmt.Println("    \"added\": {")
			keys := sortedKeys(diff.Added)
			for i, key := range keys {
				fmt.Printf("      \"%s\": \"%s\"", key, diff.Added[key])
				if i < len(keys)-1 {
					fmt.Print(",")
				}
				fmt.Println()
			}
			fmt.Print("    }")
			if len(diff.Deleted) > 0 || len(diff.Modified) > 0 {
				fmt.Print(",")
			}
			fmt.Println()
		}

		if len(diff.Deleted) > 0 {
			fmt.Println("    \"deleted\": {")
			keys := sortedKeys(diff.Deleted)
			for i, key := range keys {
				fmt.Printf("      \"%s\": \"%s\"", key, diff.Deleted[key])
				if i < len(keys)-1 {
					fmt.Print(",")
				}
				fmt.Println()
			}
			fmt.Print("    }")
			if len(diff.Modified) > 0 {
				fmt.Print(",")
			}
			fmt.Println()
		}

		if len(diff.Modified) > 0 {
			fmt.Println("    \"modified\": {")
			keys := sortedKeysModified(diff.Modified)
			for i, key := range keys {
				values := diff.Modified[key]
				fmt.Printf("      \"%s\": {\"old\": \"%s\", \"new\": \"%s\"}",
					key, maskValue(key, values[0]), maskValue(key, values[1]))
				if i < len(keys)-1 {
					fmt.Print(",")
				}
				fmt.Println()
			}
			fmt.Println("    }")
		}

		fmt.Println("  }")
	}

	fmt.Println("}")
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysModified(m map[string][2]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maskValue(key, value string) string {
	if !showValues || isSensitiveKey(key) {
		return "***"
	}
	return value
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "secret", "key", "token",
		"credential", "auth", "private",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}

	return false
}
