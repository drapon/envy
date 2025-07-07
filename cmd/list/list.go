package list

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/color"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	environment string
	source      string
	tree        bool
	filter      string
	showValues  bool
	format      string
	all         bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	Long: `List environment variables from local files, AWS, or both.

This command displays environment variables with various formatting options
including tree view, filtering, and value masking for sensitive variables.`,
	Example: `  # List variables for the default environment
  envy list
  
  # List variables for a specific environment
  envy list --env production
  
  # List variables from AWS only
  envy list --source aws
  
  # List in tree format
  envy list --tree
  
  # Filter by prefix
  envy list --filter "DB_"
  
  # Show actual values (careful with sensitive data!)
  envy list --show-values
  
  # Output as JSON
  envy list --format json
  
  # List all environments
  envy list --all`,
	RunE: runList,
}

// GetListCmd returns the list command.
func GetListCmd() *cobra.Command {
	return listCmd
}

func init() {
	root.GetRootCmd().AddCommand(listCmd)

	// Add flags specific to list command
	listCmd.Flags().StringVarP(&environment, "env", "e", "", "Specify environment")
	listCmd.Flags().StringVarP(&source, "source", "s", "both", "Source (local/aws/both)")
	listCmd.Flags().BoolVarP(&tree, "tree", "t", false, "Tree format display")
	listCmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter pattern")
	listCmd.Flags().BoolVar(&showValues, "show-values", false, "Show actual values (default: masked)")
	listCmd.Flags().StringVar(&format, "format", "text", "Output format (text/json/tree)")
	listCmd.Flags().BoolVarP(&all, "all", "a", false, "List all environments")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine which environments to list
	environments := []string{}
	if all {
		for envName := range cfg.Environments {
			environments = append(environments, envName)
		}
		sort.Strings(environments)
	} else {
		if environment == "" {
			environment = cfg.DefaultEnvironment
		}
		environments = []string{environment}
	}

	// Create AWS manager if needed
	var awsManager *aws.Manager
	if source == "aws" || source == "both" {
		awsManager, err = aws.NewManager(cfg)
		if err != nil {
			return fmt.Errorf("failed to create AWS manager: %w", err)
		}
	}

	// Process each environment
	for i, envName := range environments {
		if i > 0 {
			fmt.Println() // Add spacing between environments
		}

		if all {
			color.PrintBoldf("=== Environment: %s ===", envName)
		}

		if err := listEnvironment(ctx, cfg, awsManager, envName); err != nil {
			return fmt.Errorf("failed to list environment %s: %w", envName, err)
		}
	}

	return nil
}

func listEnvironment(ctx context.Context, cfg *config.Config, awsManager *aws.Manager, envName string) error {
	// Get environment configuration
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Collect variables from different sources
	var localVars map[string]string
	var awsVars map[string]string

	// Get local variables
	if source == "local" || source == "both" {
		envManager := env.NewManager(".")
		envFile, err := envManager.LoadFiles(envConfig.Files)
		if err != nil {
			color.PrintWarningf("Failed to load local files: %v", err)
			localVars = make(map[string]string)
		} else {
			localVars = envFile.ToMap()
		}
	}

	// Get AWS variables
	if awsManager != nil && (source == "aws" || source == "both") {
		awsVars, err = awsManager.ListEnvironmentVariables(ctx, envName)
		if err != nil {
			color.PrintWarningf("Failed to load AWS variables: %v", err)
			awsVars = make(map[string]string)
		}
	}

	// Merge and categorize variables
	allVars := make(map[string]varInfo)

	// Add local variables
	for key, value := range localVars {
		if filter != "" && !matchesFilter(key, filter) {
			continue
		}
		allVars[key] = varInfo{
			Value:     value,
			Sources:   []string{"local"},
			LocalOnly: true,
		}
	}

	// Add/update with AWS variables
	for key, value := range awsVars {
		if filter != "" && !matchesFilter(key, filter) {
			continue
		}

		if info, exists := allVars[key]; exists {
			// Variable exists in both
			info.Sources = append(info.Sources, "aws")
			info.LocalOnly = false
			info.AWSOnly = false
			info.Value = value // Use AWS value for display
			allVars[key] = info
		} else {
			// AWS only
			allVars[key] = varInfo{
				Value:   value,
				Sources: []string{"aws"},
				AWSOnly: true,
			}
		}
	}

	// Display based on format
	switch format {
	case "json":
		return displayJSON(allVars, envName)
	case "tree":
		return displayTree(allVars, envName)
	default:
		return displayText(allVars, envName)
	}
}

type varInfo struct {
	Value     string
	Sources   []string
	LocalOnly bool
	AWSOnly   bool
}

func displayText(vars map[string]varInfo, envName string) error {
	if len(vars) == 0 {
		color.PrintWarningf("No variables found")
		return nil
	}

	// Sort keys
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Display header
	if source == "both" {
		color.PrintInfof("Environment: %s (showing %s)\n", envName, source)
	} else {
		color.PrintInfof("Environment: %s (source: %s)\n", envName, source)
	}

	// Display variables
	for _, key := range keys {
		info := vars[key]

		// Source indicator
		var sourceIndicator string

		if info.LocalOnly {
			sourceIndicator = color.FormatSuccess("[local]")
		} else if info.AWSOnly {
			sourceIndicator = color.FormatInfo("[aws]")
		} else {
			sourceIndicator = color.FormatWarning("[both]")
		}

		// Display value
		displayValue := maskValue(key, info.Value)

		if source == "both" {
			fmt.Printf("%-40s = %-20s %s\n", key, displayValue, sourceIndicator)
		} else {
			fmt.Printf("%-40s = %s\n", key, displayValue)
		}
	}

	// Summary
	color.PrintBoldf("\nTotal: %d variables", len(vars))

	if source == "both" {
		localCount := 0
		awsCount := 0
		bothCount := 0

		for _, info := range vars {
			if info.LocalOnly {
				localCount++
			} else if info.AWSOnly {
				awsCount++
			} else {
				bothCount++
			}
		}

		fmt.Printf("  Local only: %d %s\n", localCount, color.FormatSuccess("(green)"))
		fmt.Printf("  AWS only: %d %s\n", awsCount, color.FormatInfo("(blue)"))
		fmt.Printf("  Both: %d %s\n", bothCount, color.FormatWarning("(yellow)"))
	}

	return nil
}

func displayTree(vars map[string]varInfo, envName string) error {
	if len(vars) == 0 {
		color.PrintWarningf("No variables found")
		return nil
	}

	color.PrintInfof("Environment: %s\n", envName)

	// Build tree structure
	root := &treeNode{
		name:     "variables",
		children: make(map[string]*treeNode),
	}

	// Add variables to tree
	for key, info := range vars {
		parts := strings.Split(key, "_")
		addToTree(root, parts, info)
	}

	// Display tree
	displayTreeNode(root, "", true)

	// Summary
	color.PrintBoldf("\nTotal: %d variables", len(vars))

	return nil
}

type treeNode struct {
	name     string
	value    *varInfo
	children map[string]*treeNode
}

func addToTree(node *treeNode, parts []string, info varInfo) {
	if len(parts) == 0 {
		return
	}

	part := parts[0]
	child, exists := node.children[part]
	if !exists {
		child = &treeNode{
			name:     part,
			children: make(map[string]*treeNode),
		}
		node.children[part] = child
	}

	if len(parts) == 1 {
		// Leaf node
		child.value = &info
	} else {
		// Continue building tree
		addToTree(child, parts[1:], info)
	}
}

func displayTreeNode(node *treeNode, indent string, isLast bool) {
	if node.name != "variables" { // Skip root node
		// Display tree connector
		if isLast {
			fmt.Print(indent + "└── ")
		} else {
			fmt.Print(indent + "├── ")
		}

		// Display node
		if node.value != nil {
			// Leaf node with value
			displayValue := maskValue(node.name, node.value.Value)

			var colorCode string
			if node.value.LocalOnly {
				colorCode = "\033[32m" // Green
			} else if node.value.AWSOnly {
				colorCode = "\033[34m" // Blue
			} else {
				colorCode = "\033[0m" // Default
			}

			fmt.Printf("%s%s = %s\033[0m\n", colorCode, node.name, displayValue)
		} else {
			// Branch node
			fmt.Printf("%s/\n", node.name)
		}
	}

	// Sort children for consistent display
	childNames := make([]string, 0, len(node.children))
	for name := range node.children {
		childNames = append(childNames, name)
	}
	sort.Strings(childNames)

	// Display children
	for i, childName := range childNames {
		child := node.children[childName]
		childIndent := indent

		if node.name != "variables" { // Skip indentation for root
			if isLast {
				childIndent += "    "
			} else {
				childIndent += "│   "
			}
		}

		displayTreeNode(child, childIndent, i == len(childNames)-1)
	}
}

func displayJSON(vars map[string]varInfo, envName string) error {
	output := map[string]interface{}{
		"environment": envName,
		"source":      source,
		"count":       len(vars),
		"variables":   make(map[string]interface{}),
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Add variables
	for _, key := range keys {
		info := vars[key]
		varData := map[string]interface{}{
			"sources": info.Sources,
		}

		if showValues {
			varData["value"] = info.Value
		} else {
			varData["value"] = maskValue(key, info.Value)
		}

		output["variables"].(map[string]interface{})[key] = varData
	}

	// Marshal and output
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func matchesFilter(key, pattern string) bool {
	// Simple contains match for now
	// Could be enhanced to support regex or glob patterns
	return strings.Contains(strings.ToLower(key), strings.ToLower(pattern))
}

func maskValue(key, value string) string {
	if showValues && !isSensitiveKey(key) {
		return value
	}

	// Mask value but show first and last character for recognition
	if len(value) <= 4 {
		return "***"
	}

	return value[:1] + "***" + value[len(value)-1:]
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "secret", "key", "token",
		"credential", "auth", "private", "cert",
		"api_key", "access_key", "secret_key",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}

	return false
}
