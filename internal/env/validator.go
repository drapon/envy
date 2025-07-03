package env

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ValidationRule represents a validation rule for environment variables
type ValidationRule struct {
	Pattern     string
	Type        string // string, int, bool, url, email, regex
	Required    bool
	MinLength   int
	MaxLength   int
	AllowedValues []string
}

// Validator validates environment variables
type Validator struct {
	rules map[string]*ValidationRule
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		rules: make(map[string]*ValidationRule),
	}
}

// AddRule adds a validation rule for a variable
func (v *Validator) AddRule(key string, rule *ValidationRule) {
	v.rules[key] = rule
}

// ValidateFile validates all variables in a file
func (v *Validator) ValidateFile(file *File) []error {
	var errors []error

	// Check required variables
	for key, rule := range v.rules {
		if rule.Required {
			if _, exists := file.Variables[key]; !exists {
				errors = append(errors, fmt.Errorf("required variable '%s' is missing", key))
			}
		}
	}

	// Validate existing variables
	for key, variable := range file.Variables {
		if rule, ok := v.rules[key]; ok {
			if err := v.validateVariable(key, variable.Value, rule); err != nil {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

// ValidateMap validates a map of variables
func (v *Validator) ValidateMap(vars map[string]string) []error {
	var errors []error

	// Check required variables
	for key, rule := range v.rules {
		if rule.Required {
			if _, exists := vars[key]; !exists {
				errors = append(errors, fmt.Errorf("required variable '%s' is missing", key))
			}
		}
	}

	// Validate existing variables
	for key, value := range vars {
		if rule, ok := v.rules[key]; ok {
			if err := v.validateVariable(key, value, rule); err != nil {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

// validateVariable validates a single variable against a rule
func (v *Validator) validateVariable(key, value string, rule *ValidationRule) error {
	// Check length constraints
	if rule.MinLength > 0 && len(value) < rule.MinLength {
		return fmt.Errorf("variable '%s' is too short (minimum %d characters)", key, rule.MinLength)
	}
	if rule.MaxLength > 0 && len(value) > rule.MaxLength {
		return fmt.Errorf("variable '%s' is too long (maximum %d characters)", key, rule.MaxLength)
	}

	// Check allowed values
	if len(rule.AllowedValues) > 0 {
		allowed := false
		for _, av := range rule.AllowedValues {
			if value == av {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("variable '%s' has invalid value '%s' (allowed: %s)", 
				key, value, strings.Join(rule.AllowedValues, ", "))
		}
	}

	// Type validation
	switch rule.Type {
	case "int":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("variable '%s' must be an integer", key)
		}
	case "bool":
		lower := strings.ToLower(value)
		if lower != "true" && lower != "false" && lower != "1" && lower != "0" {
			return fmt.Errorf("variable '%s' must be a boolean (true/false/1/0)", key)
		}
	case "url":
		if _, err := url.Parse(value); err != nil {
			return fmt.Errorf("variable '%s' must be a valid URL", key)
		}
	case "email":
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(value) {
			return fmt.Errorf("variable '%s' must be a valid email address", key)
		}
	case "regex":
		if rule.Pattern != "" {
			regex, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return fmt.Errorf("invalid regex pattern for variable '%s': %w", key, err)
			}
			if !regex.MatchString(value) {
				return fmt.Errorf("variable '%s' does not match required pattern", key)
			}
		}
	}

	return nil
}

// DefaultRules returns a set of common validation rules
func DefaultRules() map[string]*ValidationRule {
	return map[string]*ValidationRule{
		"PORT": {
			Type:      "int",
			MinLength: 1,
			MaxLength: 5,
		},
		"DATABASE_URL": {
			Type:     "url",
			Required: true,
		},
		"API_KEY": {
			Type:      "string",
			Required:  true,
			MinLength: 10,
		},
		"DEBUG": {
			Type: "bool",
		},
		"NODE_ENV": {
			Type:          "string",
			AllowedValues: []string{"development", "staging", "production", "test"},
		},
		"LOG_LEVEL": {
			Type:          "string",
			AllowedValues: []string{"debug", "info", "warn", "error", "fatal"},
		},
	}
}