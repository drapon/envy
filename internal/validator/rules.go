package validator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Rules represents validation rules for environment variables
type Rules struct {
	Required  []string                 `yaml:"required"`
	Variables map[string]*VariableRule `yaml:"variables"`
	Warnings  []WarningRule            `yaml:"warnings"`
}

// VariableRule represents validation rules for a single variable
type VariableRule struct {
	Type     string   `yaml:"type"`
	Pattern  string   `yaml:"pattern,omitempty"`
	Min      *float64 `yaml:"min,omitempty"`
	Max      *float64 `yaml:"max,omitempty"`
	Enum     []string `yaml:"enum,omitempty"`
	Required bool     `yaml:"required,omitempty"`
	Default  string   `yaml:"default,omitempty"`
}

// WarningRule represents a warning for deprecated or problematic variables
type WarningRule struct {
	Name    string `yaml:"name"`
	Message string `yaml:"message"`
}

// LoadRulesFromFile loads validation rules from a YAML file
func LoadRulesFromFile(filename string) (*Rules, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var rules Rules
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules file: %w", err)
	}

	// Initialize maps if nil
	if rules.Variables == nil {
		rules.Variables = make(map[string]*VariableRule)
	}

	// Process required variables to ensure they have rules
	for _, reqVar := range rules.Required {
		if _, exists := rules.Variables[reqVar]; !exists {
			rules.Variables[reqVar] = &VariableRule{
				Type:     "string",
				Required: true,
			}
		} else {
			rules.Variables[reqVar].Required = true
		}
	}

	return &rules, nil
}

// SaveRulesToFile saves validation rules to a YAML file
func SaveRulesToFile(rules *Rules, filename string) error {
	data, err := yaml.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}

// DefaultRules returns a set of default validation rules
func DefaultRules() *Rules {
	return &Rules{
		Required: []string{
			"NODE_ENV",
		},
		Variables: map[string]*VariableRule{
			"NODE_ENV": {
				Type:     "string",
				Enum:     []string{"development", "staging", "production", "test"},
				Required: true,
			},
			"PORT": {
				Type:    "int",
				Min:     float64Ptr(1),
				Max:     float64Ptr(65535),
				Default: "3000",
			},
			"HOST": {
				Type:    "string",
				Default: "localhost",
			},
			"DATABASE_URL": {
				Type:    "url",
				Pattern: "^(postgres|postgresql|mysql|mongodb)://",
			},
			"REDIS_URL": {
				Type:    "url",
				Pattern: "^redis://",
			},
			"API_KEY": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9_-]{16,}$",
			},
			"SECRET_KEY": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9_-]{32,}$",
			},
			"JWT_SECRET": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9_-]{32,}$",
			},
			"LOG_LEVEL": {
				Type:    "string",
				Enum:    []string{"debug", "info", "warn", "error", "fatal"},
				Default: "info",
			},
			"DEBUG": {
				Type:    "bool",
				Default: "false",
			},
			"ENABLE_HTTPS": {
				Type:    "bool",
				Default: "false",
			},
			"MAX_CONNECTIONS": {
				Type:    "int",
				Min:     float64Ptr(1),
				Max:     float64Ptr(1000),
				Default: "100",
			},
			"TIMEOUT": {
				Type:    "int",
				Min:     float64Ptr(0),
				Default: "30",
			},
			"AWS_REGION": {
				Type: "string",
				Enum: []string{
					"us-east-1", "us-east-2", "us-west-1", "us-west-2",
					"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
					"ap-northeast-1", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2",
					"ap-south-1", "sa-east-1", "ca-central-1",
				},
			},
			"AWS_ACCESS_KEY_ID": {
				Type:    "string",
				Pattern: "^[A-Z0-9]{20}$",
			},
			"AWS_SECRET_ACCESS_KEY": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9/+=]{40}$",
			},
			"SMTP_HOST": {
				Type: "string",
			},
			"SMTP_PORT": {
				Type:    "int",
				Min:     float64Ptr(1),
				Max:     float64Ptr(65535),
				Default: "587",
			},
			"SMTP_USERNAME": {
				Type: "string",
			},
			"SMTP_PASSWORD": {
				Type: "string",
			},
			"EMAIL_FROM": {
				Type: "email",
			},
			"ALLOWED_ORIGINS": {
				Type:    "string",
				Pattern: `^(\*|https?://[^\s,]+)(,(\*|https?://[^\s,]+))*$`,
			},
			"RATE_LIMIT": {
				Type:    "int",
				Min:     float64Ptr(0),
				Default: "100",
			},
			"SESSION_SECRET": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9_-]{32,}$",
			},
			"COOKIE_SECURE": {
				Type:    "bool",
				Default: "true",
			},
			"CORS_ENABLED": {
				Type:    "bool",
				Default: "false",
			},
			"CACHE_TTL": {
				Type:    "int",
				Min:     float64Ptr(0),
				Default: "3600",
			},
			"UPLOAD_MAX_SIZE": {
				Type:    "int",
				Min:     float64Ptr(0),
				Default: "10485760", // 10MB
			},
		},
		Warnings: []WarningRule{
			{
				Name:    "DEPRECATED_API_KEY",
				Message: "This variable is deprecated. Use API_KEY instead.",
			},
			{
				Name:    "OLD_DATABASE_URL",
				Message: "This variable is deprecated. Use DATABASE_URL instead.",
			},
		},
	}
}

// MergeRules merges two rule sets, with the second taking precedence
func MergeRules(base, override *Rules) *Rules {
	merged := &Rules{
		Required:  append([]string{}, base.Required...),
		Variables: make(map[string]*VariableRule),
		Warnings:  append([]WarningRule{}, base.Warnings...),
	}

	// Copy base variables
	for k, v := range base.Variables {
		merged.Variables[k] = v
	}

	// Override with new rules
	if override != nil {
		// Merge required
		for _, req := range override.Required {
			if !contains(merged.Required, req) {
				merged.Required = append(merged.Required, req)
			}
		}

		// Merge variables
		for k, v := range override.Variables {
			merged.Variables[k] = v
		}

		// Merge warnings
		merged.Warnings = append(merged.Warnings, override.Warnings...)
	}

	return merged
}

// GenerateExampleRulesFile generates an example rules file
func GenerateExampleRulesFile(filename string) error {
	example := &Rules{
		Required: []string{
			"DATABASE_URL",
			"API_KEY",
			"NODE_ENV",
		},
		Variables: map[string]*VariableRule{
			"DATABASE_URL": {
				Type:    "url",
				Pattern: "^postgres://",
			},
			"PORT": {
				Type: "int",
				Min:  float64Ptr(1),
				Max:  float64Ptr(65535),
			},
			"NODE_ENV": {
				Type: "string",
				Enum: []string{"development", "staging", "production"},
			},
			"API_KEY": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9]{32}$",
			},
			"ENABLE_DEBUG": {
				Type: "bool",
			},
			"MAX_CONNECTIONS": {
				Type:    "int",
				Min:     float64Ptr(1),
				Max:     float64Ptr(100),
				Default: "10",
			},
		},
		Warnings: []WarningRule{
			{
				Name:    "DEPRECATED_VAR",
				Message: "This variable is deprecated, use NEW_VAR instead",
			},
		},
	}

	return SaveRulesToFile(example, filename)
}

// Helper functions

func float64Ptr(f float64) *float64 {
	return &f
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
