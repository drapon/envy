package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Validator validates environment variables against defined rules
type Validator struct {
	rules *Rules
}

// New creates a new validator with the given rules
func New(rules *Rules) *Validator {
	return &Validator{
		rules: rules,
	}
}

// ValidationResult contains the results of validation
type ValidationResult struct {
	Errors       []ValidationError `json:"errors"`
	Warnings     []ValidationError `json:"warnings"`
	Fixes        []Fix             `json:"fixes"`
	AppliedFixes []Fix             `json:"applied_fixes,omitempty"`
}

// ValidationError represents a validation error or warning
type ValidationError struct {
	Variable string `json:"variable"`
	Message  string `json:"message"`
	Details  string `json:"details,omitempty"`
	Type     string `json:"type"`
}

// Fix represents a suggested or applied fix
type Fix struct {
	Variable    string  `json:"variable"`
	Type        FixType `json:"type"`
	Value       string  `json:"value,omitempty"`
	Description string  `json:"description"`
}

// FixType represents the type of fix
type FixType string

const (
	FixTypeSetDefault     FixType = "set_default"
	FixTypeCorrectValue   FixType = "correct_value"
	FixTypeRemoveVariable FixType = "remove_variable"
)

// Validate validates the given environment variables
func (v *Validator) Validate(ctx context.Context, vars map[string]string) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
		Fixes:    []Fix{},
	}

	// Check required variables from the required list
	requiredMap := make(map[string]bool)
	for _, required := range v.rules.Required {
		requiredMap[required] = true
		if _, exists := vars[required]; !exists {
			result.Errors = append(result.Errors, ValidationError{
				Variable: required,
				Message:  fmt.Sprintf("Required variable %s is missing", required),
				Type:     "missing_required",
			})
		}
	}

	// Validate each variable against its rules
	for varName, varRule := range v.rules.Variables {
		value, exists := vars[varName]
		
		// Check if required (but skip if already checked in required list)
		if varRule.Required && !exists && !requiredMap[varName] {
			result.Errors = append(result.Errors, ValidationError{
				Variable: varName,
				Message:  fmt.Sprintf("Required variable %s is missing", varName),
				Type:     "missing_required",
			})
			
			// Suggest fix if default value is available
			if varRule.Default != "" {
				result.Fixes = append(result.Fixes, Fix{
					Variable:    varName,
					Type:        FixTypeSetDefault,
					Value:       varRule.Default,
					Description: fmt.Sprintf("Set default value: %s", varRule.Default),
				})
			}
			continue
		}

		// Skip validation if variable doesn't exist and isn't required
		if !exists {
			// But suggest adding it if it has a default value
			if varRule.Default != "" {
				result.Fixes = append(result.Fixes, Fix{
					Variable:    varName,
					Type:        FixTypeSetDefault,
					Value:       varRule.Default,
					Description: fmt.Sprintf("Add optional variable with default value: %s", varRule.Default),
				})
			}
			continue
		}

		// Validate type
		if err := v.validateType(varName, value, varRule); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Variable: varName,
				Message:  err.Error(),
				Type:     "type_error",
				Details:  fmt.Sprintf("Expected type: %s", varRule.Type),
			})
		}

		// Validate pattern
		if varRule.Pattern != "" {
			if err := v.validatePattern(varName, value, varRule.Pattern); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Variable: varName,
					Message:  err.Error(),
					Type:     "pattern_error",
					Details:  fmt.Sprintf("Pattern: %s", varRule.Pattern),
				})
			}
		}

		// Validate enum
		if len(varRule.Enum) > 0 {
			if err := v.validateEnum(varName, value, varRule.Enum); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Variable: varName,
					Message:  err.Error(),
					Type:     "enum_error",
					Details:  fmt.Sprintf("Allowed values: %s", strings.Join(varRule.Enum, ", ")),
				})
			}
		}

		// Validate range for numeric types
		if varRule.Type == "int" || varRule.Type == "float" {
			if err := v.validateRange(varName, value, varRule); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Variable: varName,
					Message:  err.Error(),
					Type:     "range_error",
				})
			}
		}
	}

	// Check for warnings
	for varName := range vars {
		// Check if variable is deprecated
		for _, warning := range v.rules.Warnings {
			if warning.Name == varName {
				result.Warnings = append(result.Warnings, ValidationError{
					Variable: varName,
					Message:  warning.Message,
					Type:     "deprecated",
				})
			}
		}

		// Warn about undefined variables
		if _, defined := v.rules.Variables[varName]; !defined && !v.isInRequired(varName) {
			// Check if it's a common variable that might be missing rules
			if v.isCommonVariable(varName) {
				result.Warnings = append(result.Warnings, ValidationError{
					Variable: varName,
					Message:  fmt.Sprintf("Variable %s has no validation rules defined", varName),
					Type:     "undefined",
				})
			}
		}
	}

	return result
}

func (v *Validator) validateType(name, value string, rule *VariableRule) error {
	switch rule.Type {
	case "string":
		// String is always valid
		return nil
	
	case "int":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("variable %s must be an integer", name)
		}
	
	case "float":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("variable %s must be a float", name)
		}
	
	case "bool":
		lower := strings.ToLower(value)
		if lower != "true" && lower != "false" && lower != "1" && lower != "0" {
			return fmt.Errorf("variable %s must be a boolean (true/false/1/0)", name)
		}
	
	case "url":
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("variable %s must be a valid URL", name)
		}
	
	case "email":
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(value) {
			return fmt.Errorf("variable %s must be a valid email address", name)
		}
	
	case "json":
		var js json.RawMessage
		if err := json.Unmarshal([]byte(value), &js); err != nil {
			return fmt.Errorf("variable %s must be valid JSON", name)
		}
	
	default:
		// Unknown type, skip validation
		return nil
	}
	
	return nil
}

func (v *Validator) validatePattern(name, value, pattern string) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern for variable %s: %w", name, err)
	}
	
	if !regex.MatchString(value) {
		return fmt.Errorf("variable %s does not match required pattern", name)
	}
	
	return nil
}

func (v *Validator) validateEnum(name, value string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	
	return fmt.Errorf("variable %s has invalid value '%s'", name, value)
}

func (v *Validator) validateRange(name, value string, rule *VariableRule) error {
	if rule.Type == "int" {
		val, err := strconv.Atoi(value)
		if err != nil {
			return nil // Type validation will catch this
		}
		
		if rule.Min != nil && val < int(*rule.Min) {
			return fmt.Errorf("variable %s value %d is less than minimum %d", name, val, int(*rule.Min))
		}
		
		if rule.Max != nil && val > int(*rule.Max) {
			return fmt.Errorf("variable %s value %d is greater than maximum %d", name, val, int(*rule.Max))
		}
	} else if rule.Type == "float" {
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil // Type validation will catch this
		}
		
		if rule.Min != nil && val < *rule.Min {
			return fmt.Errorf("variable %s value %.2f is less than minimum %.2f", name, val, *rule.Min)
		}
		
		if rule.Max != nil && val > *rule.Max {
			return fmt.Errorf("variable %s value %.2f is greater than maximum %.2f", name, val, *rule.Max)
		}
	}
	
	return nil
}

func (v *Validator) isInRequired(name string) bool {
	for _, r := range v.rules.Required {
		if r == name {
			return true
		}
	}
	return false
}

func (v *Validator) isCommonVariable(name string) bool {
	common := []string{
		"PORT", "HOST", "DATABASE_URL", "API_KEY", "API_SECRET",
		"NODE_ENV", "DEBUG", "LOG_LEVEL", "SECRET_KEY",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION",
	}
	
	for _, c := range common {
		if strings.EqualFold(name, c) {
			return true
		}
	}
	
	// Check for common patterns
	lowerName := strings.ToLower(name)
	patterns := []string{"_key", "_secret", "_token", "_url", "_host", "_port"}
	for _, p := range patterns {
		if strings.Contains(lowerName, p) {
			return true
		}
	}
	
	return false
}