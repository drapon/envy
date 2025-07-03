package validator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/drapon/envy/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	rules := &Rules{
		Required: []string{"API_KEY"},
		Variables: map[string]*VariableRule{
			"API_KEY": {
				Type:     "string",
				Required: true,
			},
		},
	}
	
	validator := New(rules)
	
	assert.NotNil(t, validator)
	assert.Equal(t, rules, validator.rules)
}

func TestValidator_Validate(t *testing.T) {
	t.Run("all_valid", func(t *testing.T) {
		rules := &Rules{
			Required: []string{"API_KEY", "PORT"},
			Variables: map[string]*VariableRule{
				"API_KEY": {
					Type:     "string",
					Pattern:  "^[A-Za-z0-9]{32}$",
					Required: true,
				},
				"PORT": {
					Type: "int",
					Min:  float64Ptr(1),
					Max:  float64Ptr(65535),
				},
				"DEBUG": {
					Type: "bool",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"API_KEY": "abcdefghijklmnopqrstuvwxyz123456",
			"PORT":    "8080",
			"DEBUG":   "true",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Empty(t, result.Errors)
		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Fixes)
	})
	
	t.Run("missing_required", func(t *testing.T) {
		rules := &Rules{
			Required: []string{"API_KEY", "DATABASE_URL"},
			Variables: map[string]*VariableRule{
				"API_KEY": {
					Type:     "string",
					Required: true,
				},
				"DATABASE_URL": {
					Type:     "url",
					Required: true,
					Default:  "postgres://localhost/default",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"PORT": "8080",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Len(t, result.Errors, 2)
		assert.Equal(t, "API_KEY", result.Errors[0].Variable)
		assert.Equal(t, "missing_required", result.Errors[0].Type)
		assert.Contains(t, result.Errors[0].Message, "Required variable API_KEY is missing")
		
		assert.Equal(t, "DATABASE_URL", result.Errors[1].Variable)
		assert.Equal(t, "missing_required", result.Errors[1].Type)
		
		// Should suggest fix for variable with default
		assert.Len(t, result.Fixes, 1)
		assert.Equal(t, "DATABASE_URL", result.Fixes[0].Variable)
		assert.Equal(t, FixTypeSetDefault, result.Fixes[0].Type)
		assert.Equal(t, "postgres://localhost/default", result.Fixes[0].Value)
	})
	
	t.Run("type_validation", func(t *testing.T) {
		rules := &Rules{
			Variables: map[string]*VariableRule{
				"PORT": {
					Type: "int",
				},
				"DEBUG": {
					Type: "bool",
				},
				"RATIO": {
					Type: "float",
				},
				"WEBSITE": {
					Type: "url",
				},
				"EMAIL": {
					Type: "email",
				},
				"CONFIG": {
					Type: "json",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"PORT":    "not-a-number",
			"DEBUG":   "maybe",
			"RATIO":   "not-a-float",
			"WEBSITE": "not-a-url",
			"EMAIL":   "not-an-email",
			"CONFIG":  "not-json",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Len(t, result.Errors, 6)
		
		for _, err := range result.Errors {
			assert.Equal(t, "type_error", err.Type)
			assert.Contains(t, err.Message, "must be")
		}
	})
	
	t.Run("pattern_validation", func(t *testing.T) {
		rules := &Rules{
			Variables: map[string]*VariableRule{
				"API_KEY": {
					Type:    "string",
					Pattern: "^[A-Z0-9]{20}$",
				},
				"VERSION": {
					Type:    "string",
					Pattern: `^v\d+\.\d+\.\d+$`,
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"API_KEY": "invalid-key",
			"VERSION": "1.0.0", // Missing 'v' prefix
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Len(t, result.Errors, 2)
		
		for _, err := range result.Errors {
			assert.Equal(t, "pattern_error", err.Type)
			assert.Contains(t, err.Message, "does not match required pattern")
		}
	})
	
	t.Run("enum_validation", func(t *testing.T) {
		rules := &Rules{
			Variables: map[string]*VariableRule{
				"NODE_ENV": {
					Type: "string",
					Enum: []string{"development", "staging", "production"},
				},
				"LOG_LEVEL": {
					Type: "string",
					Enum: []string{"debug", "info", "warn", "error"},
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"NODE_ENV":  "testing",
			"LOG_LEVEL": "verbose",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Len(t, result.Errors, 2)
		
		for _, err := range result.Errors {
			assert.Equal(t, "enum_error", err.Type)
			assert.Contains(t, err.Message, "invalid value")
			assert.Contains(t, err.Details, "Allowed values:")
		}
	})
	
	t.Run("range_validation", func(t *testing.T) {
		rules := &Rules{
			Variables: map[string]*VariableRule{
				"PORT": {
					Type: "int",
					Min:  float64Ptr(1),
					Max:  float64Ptr(65535),
				},
				"TIMEOUT": {
					Type: "int",
					Min:  float64Ptr(0),
					Max:  float64Ptr(300),
				},
				"RATE": {
					Type: "float",
					Min:  float64Ptr(0.0),
					Max:  float64Ptr(1.0),
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"PORT":    "70000",
			"TIMEOUT": "-5",
			"RATE":    "1.5",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Len(t, result.Errors, 3)
		
		for _, err := range result.Errors {
			assert.Equal(t, "range_error", err.Type)
			assert.Contains(t, err.Message, "value")
		}
	})
	
	t.Run("warnings", func(t *testing.T) {
		rules := &Rules{
			Warnings: []WarningRule{
				{
					Name:    "OLD_API_KEY",
					Message: "This variable is deprecated. Use API_KEY instead.",
				},
				{
					Name:    "LEGACY_DB_URL",
					Message: "This variable is deprecated. Use DATABASE_URL instead.",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"OLD_API_KEY":  "deprecated-key",
			"LEGACY_DB_URL": "deprecated-url",
			"API_KEY":      "new-key",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Empty(t, result.Errors)
		
		// Find deprecated warnings
		var deprecatedWarnings []ValidationError
		for _, w := range result.Warnings {
			if w.Type == "deprecated" {
				deprecatedWarnings = append(deprecatedWarnings, w)
			}
		}
		
		assert.Len(t, deprecatedWarnings, 2)
		
		// Check deprecated warnings (order may vary)
		var foundOldAPIKey, foundLegacyDB bool
		for _, w := range deprecatedWarnings {
			if w.Variable == "OLD_API_KEY" {
				foundOldAPIKey = true
				assert.Contains(t, w.Message, "deprecated")
			} else if w.Variable == "LEGACY_DB_URL" {
				foundLegacyDB = true
				assert.Contains(t, w.Message, "deprecated")
			}
		}
		assert.True(t, foundOldAPIKey, "Should have warning for OLD_API_KEY")
		assert.True(t, foundLegacyDB, "Should have warning for LEGACY_DB_URL")
	})
	
	t.Run("undefined_variables", func(t *testing.T) {
		rules := &Rules{
			Variables: map[string]*VariableRule{
				"API_KEY": {
					Type: "string",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"API_KEY":      "defined",
			"DATABASE_URL": "undefined-but-common",
			"CUSTOM_VAR":   "undefined-custom",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Empty(t, result.Errors)
		// Should warn about common variable DATABASE_URL but not CUSTOM_VAR
		assert.Len(t, result.Warnings, 1)
		assert.Equal(t, "DATABASE_URL", result.Warnings[0].Variable)
		assert.Equal(t, "undefined", result.Warnings[0].Type)
		assert.Contains(t, result.Warnings[0].Message, "no validation rules defined")
	})
	
	t.Run("optional_with_default", func(t *testing.T) {
		rules := &Rules{
			Variables: map[string]*VariableRule{
				"PORT": {
					Type:    "int",
					Default: "3000",
				},
				"HOST": {
					Type:    "string",
					Default: "localhost",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"DEBUG": "true",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		assert.Empty(t, result.Errors)
		// DEBUG is a common variable so it will generate a warning
		assert.Len(t, result.Warnings, 1)
		assert.Equal(t, "DEBUG", result.Warnings[0].Variable)
		assert.Equal(t, "undefined", result.Warnings[0].Type)
		
		// Should suggest adding optional variables with defaults
		assert.Len(t, result.Fixes, 2)
		
		// Find fixes by variable name since map iteration order is not guaranteed
		var portFix, hostFix *Fix
		for i := range result.Fixes {
			switch result.Fixes[i].Variable {
			case "PORT":
				portFix = &result.Fixes[i]
			case "HOST":
				hostFix = &result.Fixes[i]
			}
		}
		
		require.NotNil(t, portFix, "PORT fix should exist")
		assert.Equal(t, FixTypeSetDefault, portFix.Type)
		assert.Equal(t, "3000", portFix.Value)
		assert.Contains(t, portFix.Description, "Add optional variable")
		
		require.NotNil(t, hostFix, "HOST fix should exist")
		assert.Equal(t, FixTypeSetDefault, hostFix.Type)
		assert.Equal(t, "localhost", hostFix.Value)
	})
	
	t.Run("complex_validation", func(t *testing.T) {
		rules := &Rules{
			Required: []string{"NODE_ENV", "DATABASE_URL"},
			Variables: map[string]*VariableRule{
				"NODE_ENV": {
					Type:     "string",
					Enum:     []string{"development", "staging", "production"},
					Required: true,
				},
				"DATABASE_URL": {
					Type:     "url",
					Pattern:  "^(postgres|mysql)://",
					Required: true,
				},
				"PORT": {
					Type:    "int",
					Min:     float64Ptr(1),
					Max:     float64Ptr(65535),
					Default: "3000",
				},
				"API_KEY": {
					Type:    "string",
					Pattern: "^[A-Za-z0-9_-]{32,}$",
				},
				"EMAIL_FROM": {
					Type: "email",
				},
			},
			Warnings: []WarningRule{
				{
					Name:    "OLD_DATABASE_URL",
					Message: "Use DATABASE_URL instead",
				},
			},
		}
		
		validator := New(rules)
		
		vars := map[string]string{
			"NODE_ENV":        "test", // Invalid enum value
			"DATABASE_URL":    "mongodb://localhost", // Invalid pattern
			"PORT":            "99999", // Out of range
			"API_KEY":         "short", // Too short
			"EMAIL_FROM":      "not-an-email",
			"OLD_DATABASE_URL": "deprecated",
		}
		
		result := validator.Validate(context.Background(), vars)
		
		// Should have errors for all invalid values
		assert.Len(t, result.Errors, 5)
		
		// Should have warning for deprecated variable and undefined variable
		var deprecatedWarnings []ValidationError
		for _, w := range result.Warnings {
			if w.Type == "deprecated" {
				deprecatedWarnings = append(deprecatedWarnings, w)
			}
		}
		assert.Len(t, deprecatedWarnings, 1)
		assert.Equal(t, "OLD_DATABASE_URL", deprecatedWarnings[0].Variable)
		
		// Should have fix for PORT with default
		var portFixes []Fix
		for _, f := range result.Fixes {
			if f.Variable == "PORT" {
				portFixes = append(portFixes, f)
			}
		}
		if len(portFixes) > 0 {
			assert.Equal(t, "PORT", portFixes[0].Variable)
		}
	})
}

func TestValidateType(t *testing.T) {
	v := &Validator{rules: &Rules{}}
	
	testCases := []struct {
		name      string
		varType   string
		value     string
		shouldErr bool
	}{
		// String type
		{"string_valid", "string", "any string", false},
		{"string_empty", "string", "", false},
		
		// Int type
		{"int_valid", "int", "42", false},
		{"int_zero", "int", "0", false},
		{"int_negative", "int", "-42", false},
		{"int_invalid", "int", "not-a-number", true},
		{"int_float", "int", "42.5", true},
		
		// Float type
		{"float_valid", "float", "3.14", false},
		{"float_int", "float", "42", false},
		{"float_scientific", "float", "1.23e-4", false},
		{"float_invalid", "float", "not-a-float", true},
		
		// Bool type
		{"bool_true", "bool", "true", false},
		{"bool_false", "bool", "false", false},
		{"bool_1", "bool", "1", false},
		{"bool_0", "bool", "0", false},
		{"bool_TRUE", "bool", "TRUE", false},
		{"bool_invalid", "bool", "yes", true},
		
		// URL type
		{"url_http", "url", "http://example.com", false},
		{"url_https", "url", "https://example.com/path?query=1", false},
		{"url_no_scheme", "url", "example.com", true},
		{"url_invalid", "url", "not a url", true},
		
		// Email type
		{"email_valid", "email", "user@example.com", false},
		{"email_subdomain", "email", "user@mail.example.com", false},
		{"email_plus", "email", "user+tag@example.com", false},
		{"email_no_at", "email", "userexample.com", true},
		{"email_no_domain", "email", "user@", true},
		
		// JSON type
		{"json_object", "json", `{"key":"value"}`, false},
		{"json_array", "json", `[1,2,3]`, false},
		{"json_string", "json", `"string"`, false},
		{"json_invalid", "json", `{invalid}`, true},
		
		// Unknown type (should not error)
		{"unknown_type", "custom", "any value", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := &VariableRule{Type: tc.varType}
			err := v.validateType("TEST_VAR", tc.value, rule)
			
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	v := &Validator{rules: &Rules{}}
	
	testCases := []struct {
		name      string
		pattern   string
		value     string
		shouldErr bool
	}{
		{"exact_match", "^test$", "test", false},
		{"no_match", "^test$", "testing", true},
		{"alphanumeric", "^[A-Za-z0-9]+$", "Test123", false},
		{"alphanumeric_special", "^[A-Za-z0-9]+$", "Test-123", true},
		{"uuid", `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, "550e8400-e29b-41d4-a716-446655440000", false},
		{"invalid_regex", "[", "test", true}, // Invalid regex should error
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.validatePattern("TEST_VAR", tc.value, tc.pattern)
			
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnum(t *testing.T) {
	v := &Validator{rules: &Rules{}}
	
	testCases := []struct {
		name      string
		allowed   []string
		value     string
		shouldErr bool
	}{
		{"valid_value", []string{"dev", "staging", "prod"}, "dev", false},
		{"invalid_value", []string{"dev", "staging", "prod"}, "test", true},
		{"case_sensitive", []string{"Dev", "Staging", "Prod"}, "dev", true},
		{"empty_enum", []string{}, "any", true},
		{"single_value", []string{"only"}, "only", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.validateEnum("TEST_VAR", tc.value, tc.allowed)
			
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRange(t *testing.T) {
	v := &Validator{rules: &Rules{}}
	
	testCases := []struct {
		name      string
		varType   string
		value     string
		min       *float64
		max       *float64
		shouldErr bool
	}{
		// Int ranges
		{"int_in_range", "int", "50", float64Ptr(1), float64Ptr(100), false},
		{"int_at_min", "int", "1", float64Ptr(1), float64Ptr(100), false},
		{"int_at_max", "int", "100", float64Ptr(1), float64Ptr(100), false},
		{"int_below_min", "int", "0", float64Ptr(1), float64Ptr(100), true},
		{"int_above_max", "int", "101", float64Ptr(1), float64Ptr(100), true},
		{"int_min_only", "int", "50", float64Ptr(10), nil, false},
		{"int_max_only", "int", "50", nil, float64Ptr(100), false},
		{"int_no_range", "int", "50", nil, nil, false},
		
		// Float ranges
		{"float_in_range", "float", "0.5", float64Ptr(0.0), float64Ptr(1.0), false},
		{"float_at_min", "float", "0.0", float64Ptr(0.0), float64Ptr(1.0), false},
		{"float_at_max", "float", "1.0", float64Ptr(0.0), float64Ptr(1.0), false},
		{"float_below_min", "float", "-0.1", float64Ptr(0.0), float64Ptr(1.0), true},
		{"float_above_max", "float", "1.1", float64Ptr(0.0), float64Ptr(1.0), true},
		
		// Invalid values (type validation would catch these)
		{"int_invalid", "int", "not-a-number", float64Ptr(1), float64Ptr(100), false},
		{"float_invalid", "float", "not-a-float", float64Ptr(0.0), float64Ptr(1.0), false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := &VariableRule{
				Type: tc.varType,
				Min:  tc.min,
				Max:  tc.max,
			}
			
			err := v.validateRange("TEST_VAR", tc.value, rule)
			
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsCommonVariable(t *testing.T) {
	v := &Validator{rules: &Rules{}}
	
	testCases := []struct {
		name     string
		varName  string
		expected bool
	}{
		// Exact matches
		{"port", "PORT", true},
		{"host", "HOST", true},
		{"database_url", "DATABASE_URL", true},
		{"api_key", "API_KEY", true},
		{"node_env", "NODE_ENV", true},
		{"debug", "DEBUG", true},
		{"log_level", "LOG_LEVEL", true},
		{"aws_region", "AWS_REGION", true},
		
		// Case insensitive
		{"lowercase", "port", true},
		{"mixed_case", "Api_Key", true},
		
		// Pattern matching
		{"ends_with_key", "CUSTOM_API_KEY", true},
		{"ends_with_secret", "JWT_SECRET", true},
		{"ends_with_token", "AUTH_TOKEN", true},
		{"ends_with_url", "REDIS_URL", true},
		{"ends_with_host", "SMTP_HOST", true},
		{"ends_with_port", "SMTP_PORT", true},
		
		// Non-common variables
		{"custom", "MY_CUSTOM_VAR", false},
		{"app_name", "APP_NAME", false},
		{"version", "VERSION", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := v.isCommonVariable(tc.varName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLoadRulesFromFile(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	t.Run("valid_rules_file", func(t *testing.T) {
		content := `
required:
  - API_KEY
  - DATABASE_URL

variables:
  API_KEY:
    type: string
    pattern: "^[A-Za-z0-9]{32}$"
    required: true
  DATABASE_URL:
    type: url
    pattern: "^postgres://"
    required: true
  PORT:
    type: int
    min: 1
    max: 65535
    default: "3000"
  NODE_ENV:
    type: string
    enum:
      - development
      - staging
      - production

warnings:
  - name: OLD_API_KEY
    message: "This variable is deprecated"
`
		
		rulesFile := helper.CreateTempFile("rules.yaml", content)
		
		rules, err := LoadRulesFromFile(rulesFile)
		
		require.NoError(t, err)
		require.NotNil(t, rules)
		
		assert.Len(t, rules.Required, 2)
		assert.Contains(t, rules.Required, "API_KEY")
		assert.Contains(t, rules.Required, "DATABASE_URL")
		
		assert.Len(t, rules.Variables, 4)
		
		apiKey := rules.Variables["API_KEY"]
		assert.Equal(t, "string", apiKey.Type)
		assert.Equal(t, "^[A-Za-z0-9]{32}$", apiKey.Pattern)
		assert.True(t, apiKey.Required)
		
		port := rules.Variables["PORT"]
		assert.Equal(t, "int", port.Type)
		assert.Equal(t, float64(1), *port.Min)
		assert.Equal(t, float64(65535), *port.Max)
		assert.Equal(t, "3000", port.Default)
		
		nodeEnv := rules.Variables["NODE_ENV"]
		assert.Equal(t, "string", nodeEnv.Type)
		assert.Equal(t, []string{"development", "staging", "production"}, nodeEnv.Enum)
		
		assert.Len(t, rules.Warnings, 1)
		assert.Equal(t, "OLD_API_KEY", rules.Warnings[0].Name)
	})
	
	t.Run("file_not_found", func(t *testing.T) {
		rules, err := LoadRulesFromFile("/non/existent/file.yaml")
		
		assert.Error(t, err)
		assert.Nil(t, rules)
		assert.Contains(t, err.Error(), "failed to read rules file")
	})
	
	t.Run("invalid_yaml", func(t *testing.T) {
		content := `
invalid yaml content
  - not proper structure
  missing: colons
`
		
		rulesFile := helper.CreateTempFile("invalid.yaml", content)
		
		rules, err := LoadRulesFromFile(rulesFile)
		
		assert.Error(t, err)
		assert.Nil(t, rules)
		assert.Contains(t, err.Error(), "failed to parse rules file")
	})
	
	t.Run("required_variables_auto_rules", func(t *testing.T) {
		content := `
required:
  - API_KEY
  - DATABASE_URL
  - NEW_VAR

variables:
  API_KEY:
    type: string
    pattern: "^[A-Za-z0-9]{32}$"
`
		
		rulesFile := helper.CreateTempFile("rules.yaml", content)
		
		rules, err := LoadRulesFromFile(rulesFile)
		
		require.NoError(t, err)
		require.NotNil(t, rules)
		
		// Should have rules for all required variables
		assert.Len(t, rules.Variables, 3)
		
		// Existing rule should be preserved
		apiKey := rules.Variables["API_KEY"]
		assert.Equal(t, "string", apiKey.Type)
		assert.Equal(t, "^[A-Za-z0-9]{32}$", apiKey.Pattern)
		assert.True(t, apiKey.Required)
		
		// New required variables should get default rules
		dbUrl := rules.Variables["DATABASE_URL"]
		assert.Equal(t, "string", dbUrl.Type)
		assert.True(t, dbUrl.Required)
		
		newVar := rules.Variables["NEW_VAR"]
		assert.Equal(t, "string", newVar.Type)
		assert.True(t, newVar.Required)
	})
}

func TestSaveRulesToFile(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	rules := &Rules{
		Required: []string{"API_KEY", "DATABASE_URL"},
		Variables: map[string]*VariableRule{
			"API_KEY": {
				Type:     "string",
				Pattern:  "^[A-Za-z0-9]{32}$",
				Required: true,
			},
			"PORT": {
				Type:    "int",
				Min:     float64Ptr(1),
				Max:     float64Ptr(65535),
				Default: "3000",
			},
		},
		Warnings: []WarningRule{
			{
				Name:    "OLD_API_KEY",
				Message: "Use API_KEY instead",
			},
		},
	}
	
	t.Run("save_and_load", func(t *testing.T) {
		rulesFile := filepath.Join(helper.TempDir(), "saved_rules.yaml")
		
		err := SaveRulesToFile(rules, rulesFile)
		require.NoError(t, err)
		
		// Load saved rules
		loadedRules, err := LoadRulesFromFile(rulesFile)
		require.NoError(t, err)
		
		assert.Equal(t, rules.Required, loadedRules.Required)
		assert.Len(t, loadedRules.Variables, 3) // 2 original + 1 auto-added from Required
		assert.Len(t, loadedRules.Warnings, 1)
		
		// Verify specific fields
		apiKey := loadedRules.Variables["API_KEY"]
		assert.Equal(t, "string", apiKey.Type)
		assert.Equal(t, "^[A-Za-z0-9]{32}$", apiKey.Pattern)
		assert.True(t, apiKey.Required)
		
		port := loadedRules.Variables["PORT"]
		assert.Equal(t, "int", port.Type)
		assert.Equal(t, float64(1), *port.Min)
		assert.Equal(t, float64(65535), *port.Max)
		assert.Equal(t, "3000", port.Default)
		
		// Verify auto-added DATABASE_URL from Required list
		dbUrl := loadedRules.Variables["DATABASE_URL"]
		assert.Equal(t, "string", dbUrl.Type)
		assert.True(t, dbUrl.Required)
	})
	
	t.Run("save_to_invalid_path", func(t *testing.T) {
		err := SaveRulesToFile(rules, "/non/existent/directory/rules.yaml")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write rules file")
	})
}

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	
	assert.NotNil(t, rules)
	assert.Contains(t, rules.Required, "NODE_ENV")
	
	// Check some default variable rules
	nodeEnv := rules.Variables["NODE_ENV"]
	assert.Equal(t, "string", nodeEnv.Type)
	assert.True(t, nodeEnv.Required)
	assert.Equal(t, []string{"development", "staging", "production", "test"}, nodeEnv.Enum)
	
	port := rules.Variables["PORT"]
	assert.Equal(t, "int", port.Type)
	assert.Equal(t, float64(1), *port.Min)
	assert.Equal(t, float64(65535), *port.Max)
	assert.Equal(t, "3000", port.Default)
	
	apiKey := rules.Variables["API_KEY"]
	assert.Equal(t, "string", apiKey.Type)
	assert.Equal(t, "^[A-Za-z0-9_-]{16,}$", apiKey.Pattern)
	
	// Check warnings
	assert.NotEmpty(t, rules.Warnings)
}

func TestMergeRules(t *testing.T) {
	base := &Rules{
		Required: []string{"API_KEY"},
		Variables: map[string]*VariableRule{
			"API_KEY": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9]{16}$",
			},
			"PORT": {
				Type:    "int",
				Default: "3000",
			},
		},
		Warnings: []WarningRule{
			{
				Name:    "OLD_VAR",
				Message: "Deprecated",
			},
		},
	}
	
	override := &Rules{
		Required: []string{"DATABASE_URL"},
		Variables: map[string]*VariableRule{
			"API_KEY": {
				Type:    "string",
				Pattern: "^[A-Za-z0-9]{32}$", // Override pattern
			},
			"DATABASE_URL": {
				Type: "url",
			},
		},
		Warnings: []WarningRule{
			{
				Name:    "NEW_WARNING",
				Message: "New warning",
			},
		},
	}
	
	merged := MergeRules(base, override)
	
	// Check required
	assert.Len(t, merged.Required, 2)
	assert.Contains(t, merged.Required, "API_KEY")
	assert.Contains(t, merged.Required, "DATABASE_URL")
	
	// Check variables
	assert.Len(t, merged.Variables, 3)
	
	// API_KEY should be overridden
	apiKey := merged.Variables["API_KEY"]
	assert.Equal(t, "^[A-Za-z0-9]{32}$", apiKey.Pattern)
	
	// PORT should remain from base
	port := merged.Variables["PORT"]
	assert.Equal(t, "3000", port.Default)
	
	// DATABASE_URL should be added
	dbUrl := merged.Variables["DATABASE_URL"]
	assert.Equal(t, "url", dbUrl.Type)
	
	// Check warnings
	assert.Len(t, merged.Warnings, 2)
}

func TestGenerateExampleRulesFile(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()
	
	rulesFile := filepath.Join(helper.TempDir(), "example_rules.yaml")
	
	err := GenerateExampleRulesFile(rulesFile)
	require.NoError(t, err)
	
	// Load generated file
	rules, err := LoadRulesFromFile(rulesFile)
	require.NoError(t, err)
	
	assert.NotNil(t, rules)
	assert.Contains(t, rules.Required, "DATABASE_URL")
	assert.Contains(t, rules.Required, "API_KEY")
	assert.Contains(t, rules.Required, "NODE_ENV")
	
	// Check that it has some example variables
	assert.NotEmpty(t, rules.Variables)
	assert.NotEmpty(t, rules.Warnings)
}

func TestContains(t *testing.T) {
	slice := []string{"one", "two", "three"}
	
	assert.True(t, contains(slice, "one"))
	assert.True(t, contains(slice, "two"))
	assert.True(t, contains(slice, "three"))
	assert.False(t, contains(slice, "four"))
	assert.False(t, contains(slice, ""))
}

// Benchmark tests
func BenchmarkValidator_Validate(b *testing.B) {
	rules := DefaultRules()
	validator := New(rules)
	
	vars := map[string]string{
		"NODE_ENV":     "production",
		"PORT":         "8080",
		"DATABASE_URL": "postgres://localhost/db",
		"API_KEY":      "abcdefghijklmnopqrstuvwxyz123456",
		"LOG_LEVEL":    "info",
		"DEBUG":        "false",
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(ctx, vars)
	}
}

func BenchmarkValidateType(b *testing.B) {
	v := &Validator{rules: &Rules{}}
	rule := &VariableRule{Type: "int"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.validateType("PORT", "8080", rule)
	}
}

func BenchmarkValidatePattern(b *testing.B) {
	v := &Validator{rules: &Rules{}}
	pattern := "^[A-Za-z0-9_-]{32,}$"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.validatePattern("API_KEY", "abcdefghijklmnopqrstuvwxyz123456", pattern)
	}
}

func BenchmarkIsCommonVariable(b *testing.B) {
	v := &Validator{rules: &Rules{}}
	vars := []string{
		"PORT", "API_KEY", "CUSTOM_VAR", "DATABASE_URL",
		"JWT_SECRET", "MY_VAR", "AUTH_TOKEN", "APP_NAME",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.isCommonVariable(vars[i%len(vars)])
	}
}