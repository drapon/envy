package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/color"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	environment string
	file        string
	rules       string
	strict      bool
	format      string
	fix         bool
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate environment variables",
	Long: `Validate environment variables against defined rules and schemas.

This command checks that all required environment variables are present,
properly formatted, and meet any defined validation criteria.`,
	Example: `  # Validate current environment
  envy validate
  
  # Validate a specific environment
  envy validate --env production
  
  # Validate a specific env file
  envy validate --file .env.production
  
  # Validate with custom rules
  envy validate --rules .envy-rules.yaml
  
  # Strict validation (fail on warnings)
  envy validate --strict
  
  # Auto-fix issues where possible
  envy validate --fix
  
  # Output as JSON
  envy validate --format json`,
	RunE: runValidate,
}

func init() {
	root.GetRootCmd().AddCommand(validateCmd)
	
	// Add flags specific to validate command
	validateCmd.Flags().StringVarP(&environment, "env", "e", "", "Environment to validate")
	validateCmd.Flags().StringVarP(&file, "file", "f", "", "Environment file to validate")
	validateCmd.Flags().StringVarP(&rules, "rules", "r", "", "Custom validation rules file (.envy-rules.yaml)")
	validateCmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors")
	validateCmd.Flags().StringVar(&format, "format", "text", "Output format (text/json)")
	validateCmd.Flags().BoolVar(&fix, "fix", false, "Auto-fix issues where possible")
}

func runValidate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine what to validate
	var envFiles []string
	var envName string

	if file != "" {
		// Validate specific file
		envFiles = []string{file}
		envName = "custom"
	} else {
		// Validate environment
		if environment == "" {
			environment = cfg.DefaultEnvironment
		}
		envName = environment

		envConfig, err := cfg.GetEnvironment(environment)
		if err != nil {
			return fmt.Errorf("failed to get environment configuration: %w", err)
		}
		envFiles = envConfig.Files
	}

	// Load validation rules
	var validationRules *validator.Rules
	if rules != "" {
		// Load custom rules file
		validationRules, err = validator.LoadRulesFromFile(rules)
		if err != nil {
			return fmt.Errorf("failed to load rules file: %w", err)
		}
	} else {
		// Check for default rules file
		defaultRulesFile := ".envy-rules.yaml"
		if _, err := os.Stat(defaultRulesFile); err == nil {
			validationRules, err = validator.LoadRulesFromFile(defaultRulesFile)
			if err != nil {
				return fmt.Errorf("failed to load default rules file: %w", err)
			}
		} else {
			// Use built-in default rules
			validationRules = validator.DefaultRules()
		}
	}

	// Load environment variables
	envManager := env.NewManager(".")
	envFile, err := envManager.LoadFiles(envFiles)
	if err != nil {
		return fmt.Errorf("failed to load environment files: %w", err)
	}

	// Create validator
	v := validator.New(validationRules)

	// Validate
	result := v.Validate(ctx, envFile.ToMap())

	// Apply fixes if requested
	if fix && len(result.Fixes) > 0 {
		fixes := applyFixes(envFile, result.Fixes)
		if len(fixes) > 0 {
			// Save the fixed file
			for _, filePath := range envFiles {
				if err := envManager.SaveFile(filePath, envFile); err != nil {
					return fmt.Errorf("failed to save fixed file %s: %w", filePath, err)
				}
			}
			result.AppliedFixes = fixes
		}
	}

	// Check if validation failed
	hasErrors := len(result.Errors) > 0
	hasWarnings := len(result.Warnings) > 0

	if strict && hasWarnings {
		hasErrors = true
	}

	// Output results
	switch format {
	case "json":
		if err := outputJSON(result, envName); err != nil {
			return err
		}
	default:
		outputText(result, envName)
	}

	// Exit with error code if validation failed
	if hasErrors {
		os.Exit(1)
	}

	return nil
}

func applyFixes(envFile *env.File, fixes []validator.Fix) []validator.Fix {
	applied := []validator.Fix{}

	for _, fix := range fixes {
		switch fix.Type {
		case validator.FixTypeSetDefault:
			if _, exists := envFile.Variables[fix.Variable]; !exists {
				envFile.Variables[fix.Variable] = &env.Variable{
					Key:   fix.Variable,
					Value: fix.Value,
				}
				applied = append(applied, fix)
			}
		case validator.FixTypeCorrectValue:
			if v, exists := envFile.Variables[fix.Variable]; exists {
				v.Value = fix.Value
				applied = append(applied, fix)
			}
		case validator.FixTypeRemoveVariable:
			delete(envFile.Variables, fix.Variable)
			applied = append(applied, fix)
		}
	}

	return applied
}

func outputText(result *validator.ValidationResult, envName string) {
	color.PrintInfo("Validating environment: %s\n", envName)

	// Summary
	errorCount := len(result.Errors)
	warningCount := len(result.Warnings)
	
	if errorCount == 0 && warningCount == 0 {
		color.PrintSuccess("✅ All validation checks passed!")
		return
	}

	// Display errors
	if errorCount > 0 {
		color.PrintError("❌ Errors (%d):", errorCount)
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", color.FormatError(err.Message))
			if err.Details != "" {
				fmt.Printf("    %s\n", err.Details)
			}
		}
		fmt.Println()
	}

	// Display warnings
	if warningCount > 0 {
		color.PrintWarning("⚠️  Warnings (%d):", warningCount)
		for _, warn := range result.Warnings {
			fmt.Printf("  - %s\n", color.FormatWarning(warn.Message))
			if warn.Details != "" {
				fmt.Printf("    %s\n", warn.Details)
			}
		}
		fmt.Println()
	}

	// Display available fixes
	if len(result.Fixes) > 0 && !fix {
		color.PrintInfo("💡 Available fixes (%d):", len(result.Fixes))
		for _, f := range result.Fixes {
			fmt.Printf("  - %s: %s\n", f.Variable, f.Description)
		}
		color.PrintInfo("\nRun with --fix to apply these fixes automatically.")
		fmt.Println()
	}

	// Display applied fixes
	if len(result.AppliedFixes) > 0 {
		color.PrintSuccess("✨ Applied fixes (%d):", len(result.AppliedFixes))
		for _, f := range result.AppliedFixes {
			fmt.Printf("  - %s: %s\n", f.Variable, f.Description)
		}
		fmt.Println()
	}

	// Summary line
	status := "FAILED"
	if errorCount == 0 {
		if warningCount > 0 && strict {
			status = "FAILED (strict mode)"
		} else {
			status = "PASSED with warnings"
		}
	}
	
	if errorCount == 0 && warningCount == 0 {
		color.PrintSuccess("Validation %s: %d errors, %d warnings", status, errorCount, warningCount)
	} else if errorCount > 0 {
		color.PrintError("Validation %s: %d errors, %d warnings", status, errorCount, warningCount)
	} else {
		color.PrintWarning("Validation %s: %d errors, %d warnings", status, errorCount, warningCount)
	}
}

func outputJSON(result *validator.ValidationResult, envName string) error {
	output := map[string]interface{}{
		"environment": envName,
		"status":      "passed",
		"errors":      result.Errors,
		"warnings":    result.Warnings,
		"fixes":       result.Fixes,
		"applied_fixes": result.AppliedFixes,
		"summary": map[string]int{
			"errors":   len(result.Errors),
			"warnings": len(result.Warnings),
			"fixes":    len(result.Fixes),
		},
	}

	if len(result.Errors) > 0 || (strict && len(result.Warnings) > 0) {
		output["status"] = "failed"
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}