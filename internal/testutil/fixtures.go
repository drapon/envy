package testutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TestFixtures provides common test data and fixtures
type TestFixtures struct{}

// NewTestFixtures creates a new test fixtures instance
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{}
}

// SimpleEnvContent returns simple .env file content
func (f *TestFixtures) SimpleEnvContent() string {
	return `APP_NAME=test-app
DEBUG=true
PORT=8080`
}

// ComplexEnvContent returns complex .env file content with various formats
func (f *TestFixtures) ComplexEnvContent() string {
	return `# Application configuration
APP_NAME=test-application
DEBUG=true
LOG_LEVEL=debug

# Database configuration
DATABASE_URL=postgres://user:password@localhost:5432/testdb
DATABASE_POOL_SIZE=10
DATABASE_TIMEOUT=30s

# Redis configuration
REDIS_URL=redis://localhost:6379/0
REDIS_PASSWORD=""
REDIS_TIMEOUT=5s

# API configuration
API_HOST=localhost
API_PORT=8080
API_VERSION=v1

# Security
JWT_SECRET=super-secret-jwt-key-for-testing
API_KEY=test-api-key-12345
ENCRYPTION_PASSWORD=test-encryption-password

# Feature flags
FEATURE_NOTIFICATIONS=true
FEATURE_ANALYTICS=false
FEATURE_BETA_UI=true

# External services
SENDGRID_API_KEY=sg.test-sendgrid-api-key
STRIPE_SECRET_KEY=sk_test_stripe_secret_key
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Environment specific
NODE_ENV=test
RACK_ENV=test
RAILS_ENV=test`
}

// EnvContentWithComments returns .env content with inline comments
func (f *TestFixtures) EnvContentWithComments() string {
	return `# Main application settings
APP_NAME=test-app  # Application name
DEBUG=true         # Enable debug mode
PORT=8080          # Server port

# Database settings
DATABASE_URL=postgres://localhost/db  # Primary database
REDIS_URL=redis://localhost:6379      # Cache database

# API keys (keep secret)
API_KEY=secret-123                     # External API key
JWT_SECRET=jwt-secret-456              # JWT signing secret`
}

// EnvContentWithQuotes returns .env content with various quote formats
func (f *TestFixtures) EnvContentWithQuotes() string {
	return `UNQUOTED_VAR=simple_value
DOUBLE_QUOTED_VAR="double quoted value"
SINGLE_QUOTED_VAR='single quoted value'
EMPTY_QUOTED_VAR=""
EMPTY_SINGLE_QUOTED_VAR=''
SPACES_IN_QUOTES="  value with spaces  "
SPECIAL_CHARS="value with !@#$%^&*() chars"
MULTILINE_VALUE="line1
line2
line3"
EQUALS_IN_VALUE="key=value inside"
HASH_IN_VALUE="value with # hash inside"`
}

// EnvContentWithSpecialCases returns .env content with edge cases
func (f *TestFixtures) EnvContentWithSpecialCases() string {
	return `# Edge cases for parser testing
EMPTY_VAR=
ONLY_SPACES_VAR=   
ONLY_TABS_VAR=			
NEWLINES_VAR="
"
BACKSLASH_VAR=path\\to\\file
UNICODE_VAR=测试值
EMOJI_VAR=🚀🎉✨
BASE64_VAR=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9
URL_VAR=https://example.com/path?param=value&other=123
JSON_VAR={"key":"value","number":123,"bool":true}
ARRAY_VAR=item1,item2,item3
EXPORT_VAR=export VALUE=something`
}

// EnvContentMalformed returns malformed .env content for error testing
func (f *TestFixtures) EnvContentMalformed() string {
	return `# Malformed content for error testing
MISSING_EQUALS_SIGN
=MISSING_KEY
INVALID-KEY-NAME=value
123_NUMERIC_START=value
KEY WITH SPACES=value
KEY=value=extra=equals
UNTERMINATED_QUOTE="value without closing quote
KEY="unclosed 'mixed quotes"`
}

// EnvContentLarge returns large .env content for performance testing
func (f *TestFixtures) EnvContentLarge(size int) string {
	var builder strings.Builder
	builder.WriteString("# Large environment file for testing\n")

	for i := 0; i < size; i++ {
		builder.WriteString(fmt.Sprintf("VAR_%d=value_%d_%s\n", i, i, strings.Repeat("x", 50)))
	}

	return builder.String()
}

// AWSParameterStoreData returns sample AWS Parameter Store data
func (f *TestFixtures) AWSParameterStoreData() map[string]string {
	return map[string]string{
		"/myapp/prod/APP_NAME":        "production-app",
		"/myapp/prod/DEBUG":           "false",
		"/myapp/prod/DATABASE_URL":    "postgres://prod.example.com/db",
		"/myapp/prod/API_KEY":         "prod-api-key-xyz",
		"/myapp/prod/PORT":            "80",
		"/myapp/prod/REDIS_URL":       "redis://prod-redis.example.com:6379",
		"/myapp/prod/JWT_SECRET":      "prod-jwt-secret-abc",
		"/myapp/staging/APP_NAME":     "staging-app",
		"/myapp/staging/DEBUG":        "true",
		"/myapp/staging/DATABASE_URL": "postgres://staging.example.com/db",
		"/myapp/staging/API_KEY":      "staging-api-key-123",
		"/myapp/staging/PORT":         "8080",
		"/myapp/dev/APP_NAME":         "dev-app",
		"/myapp/dev/DEBUG":            "true",
		"/myapp/dev/DATABASE_URL":     "postgres://localhost/dev_db",
		"/myapp/dev/API_KEY":          "dev-api-key-local",
		"/myapp/dev/PORT":             "3000",
	}
}

// AWSSecretsManagerData returns sample AWS Secrets Manager data
func (f *TestFixtures) AWSSecretsManagerData() map[string]map[string]string {
	return map[string]map[string]string{
		"myapp-prod-secrets": {
			"DATABASE_PASSWORD": "super-secret-prod-password",
			"JWT_SECRET":        "prod-jwt-signing-secret",
			"API_SECRET":        "prod-api-secret-key",
			"ENCRYPTION_KEY":    "prod-encryption-master-key",
		},
		"myapp-staging-secrets": {
			"DATABASE_PASSWORD": "staging-password-123",
			"JWT_SECRET":        "staging-jwt-secret",
			"API_SECRET":        "staging-api-secret",
			"ENCRYPTION_KEY":    "staging-encryption-key",
		},
		"myapp-dev-secrets": {
			"DATABASE_PASSWORD": "dev-password-local",
			"JWT_SECRET":        "dev-jwt-secret",
			"API_SECRET":        "dev-api-secret",
			"ENCRYPTION_KEY":    "dev-encryption-key",
		},
	}
}

// ConfigYAML returns sample .envyrc configuration
func (f *TestFixtures) ConfigYAML() string {
	return `project: myapp
default_environment: dev

aws:
  service: parameter_store
  region: us-east-1
  profile: default

cache:
  enabled: true
  type: hybrid
  ttl: 1h
  max_size: 100MB
  max_entries: 1000

memory:
  enabled: true
  pool_enabled: true
  monitoring_enabled: true
  string_pool_size: 1024
  byte_pool_size: 65536
  map_pool_size: 100
  gc_interval: 30s
  memory_threshold: 104857600

performance:
  batch_size: 50
  worker_count: 4
  streaming_enabled: true
  buffer_size: 8192
  max_line_size: 65536

environments:
  dev:
    files:
      - .env.dev
      - .env.local
    path: /myapp/dev/
    use_secrets_manager: false
    
  staging:
    files:
      - .env.staging
    path: /myapp/staging/
    use_secrets_manager: false
    
  prod:
    files:
      - .env.prod
    path: /myapp/prod/
    use_secrets_manager: true`
}

// ConfigYAMLMinimal returns minimal .envyrc configuration
func (f *TestFixtures) ConfigYAMLMinimal() string {
	return `project: testapp
default_environment: dev

environments:
  dev:
    files:
      - .env
    path: /testapp/dev/`
}

// ConfigYAMLInvalid returns invalid .envyrc configuration for error testing
func (f *TestFixtures) ConfigYAMLInvalid() string {
	return `project:  # missing project name
default_environment: dev

aws:
  service: invalid_service  # invalid service name
  region:  # missing region

environments:
  dev:
    files: []  # empty files array
    path:      # missing path`
}

// TableTestCases returns common table test case structures
func (f *TestFixtures) TableTestCases() interface{} {
	return struct {
		ValidEnvLines []struct {
			Input    string
			Expected map[string]string
		}
		InvalidEnvLines []struct {
			Input       string
			ExpectError bool
		}
		ConfigValidation []struct {
			Config     string
			Valid      bool
			ErrorField string
		}
	}{
		ValidEnvLines: []struct {
			Input    string
			Expected map[string]string
		}{
			{
				Input: "KEY=value",
				Expected: map[string]string{
					"KEY": "value",
				},
			},
			{
				Input: `KEY="quoted value"`,
				Expected: map[string]string{
					"KEY": "quoted value",
				},
			},
			{
				Input: "KEY=value # comment",
				Expected: map[string]string{
					"KEY": "value",
				},
			},
			{
				Input: "EMPTY_KEY=",
				Expected: map[string]string{
					"EMPTY_KEY": "",
				},
			},
		},
		InvalidEnvLines: []struct {
			Input       string
			ExpectError bool
		}{
			{
				Input:       "INVALID LINE WITHOUT EQUALS",
				ExpectError: false, // Should be ignored, not error
			},
			{
				Input:       "=VALUE_WITHOUT_KEY",
				ExpectError: false, // Should be ignored, not error
			},
			{
				Input:       "123_INVALID_KEY=value",
				ExpectError: false, // Should be ignored, not error
			},
		},
		ConfigValidation: []struct {
			Config     string
			Valid      bool
			ErrorField string
		}{
			{
				Config: `project: test
default_environment: dev
environments:
  dev:
    files: [".env"]
    path: "/test/dev/"`,
				Valid: true,
			},
			{
				Config: `project: ""
default_environment: dev`,
				Valid:      false,
				ErrorField: "project",
			},
		},
	}
}

// SensitiveVariableNames returns common sensitive variable name patterns
func (f *TestFixtures) SensitiveVariableNames() []string {
	return []string{
		"PASSWORD",
		"SECRET",
		"API_KEY",
		"API_SECRET",
		"JWT_SECRET",
		"JWT_TOKEN",
		"ACCESS_TOKEN",
		"REFRESH_TOKEN",
		"PRIVATE_KEY",
		"CERTIFICATE",
		"CREDENTIAL",
		"ENCRYPTION_KEY",
		"ENCRYPTION_PASSWORD",
		"OAUTH_SECRET",
		"SESSION_SECRET",
		"SIGNING_KEY",
		"AUTH_TOKEN",
		"DATABASE_PASSWORD",
		"DB_PASSWORD",
		"REDIS_PASSWORD",
		"SMTP_PASSWORD",
		"FTP_PASSWORD",
		"SSH_PRIVATE_KEY",
		"TLS_KEY",
		"SSL_KEY",
		"AWS_SECRET_ACCESS_KEY",
		"GITHUB_TOKEN",
		"SLACK_TOKEN",
		"WEBHOOK_SECRET",
		"MASTER_KEY",
		"ADMIN_PASSWORD",
	}
}

// NonSensitiveVariableNames returns common non-sensitive variable names
func (f *TestFixtures) NonSensitiveVariableNames() []string {
	return []string{
		"APP_NAME",
		"APP_VERSION",
		"DEBUG",
		"NODE_ENV",
		"RACK_ENV",
		"RAILS_ENV",
		"PORT",
		"HOST",
		"URL",
		"ENDPOINT",
		"TIMEOUT",
		"RETRY_COUNT",
		"LOG_LEVEL",
		"DATABASE_URL",
		"REDIS_URL",
		"QUEUE_URL",
		"API_URL",
		"API_VERSION",
		"REGION",
		"TIMEZONE",
		"LOCALE",
		"LANGUAGE",
		"FEATURE_FLAG",
		"CACHE_TTL",
		"BATCH_SIZE",
		"WORKER_COUNT",
		"MAX_CONNECTIONS",
		"BUFFER_SIZE",
		"PAGE_SIZE",
		"DEFAULT_LIMIT",
	}
}

// JSONSecretValue returns a JSON formatted secret value
func (f *TestFixtures) JSONSecretValue() string {
	data := map[string]interface{}{
		"database": map[string]string{
			"host":     "prod-db.example.com",
			"port":     "5432",
			"username": "app_user",
			"password": "super-secret-password",
			"database": "production_db",
		},
		"api": map[string]string{
			"key":    "prod-api-key-xyz",
			"secret": "prod-api-secret-abc",
		},
		"jwt": map[string]string{
			"secret":     "jwt-signing-secret",
			"expires_in": "24h",
		},
	}

	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}

// Base64EncodedValue returns a base64 encoded value
func (f *TestFixtures) Base64EncodedValue() string {
	return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
}

// URLValue returns a sample URL value
func (f *TestFixtures) URLValue() string {
	return "https://api.example.com/v1/endpoint?token=abc123&format=json"
}

// MultilineValue returns a multiline value
func (f *TestFixtures) MultilineValue() string {
	return `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC7VJTUt9Us8cKB
wEiOfQIVJcSxNH7vLxdJFpRZ5P5QZQP7tHR5hNZw8CKRqXb8JXCdPvZG2YYxCtg
-----END PRIVATE KEY-----`
}

// UnicodeValue returns a value with unicode characters
func (f *TestFixtures) UnicodeValue() string {
	return "测试应用程序 🚀 Application de test 🎉 Тестовое приложение ✨"
}

// GenerateTestData generates test data with specified parameters
func (f *TestFixtures) GenerateTestData(opts TestDataOptions) map[string]string {
	data := make(map[string]string)

	// Add regular variables
	for i := 0; i < opts.RegularVars; i++ {
		key := fmt.Sprintf("VAR_%d", i)
		value := fmt.Sprintf("value_%d", i)
		if opts.LongValues {
			value += "_" + strings.Repeat("x", opts.ValueLength)
		}
		data[key] = value
	}

	// Add sensitive variables
	sensitiveNames := f.SensitiveVariableNames()
	for i := 0; i < opts.SensitiveVars && i < len(sensitiveNames); i++ {
		key := sensitiveNames[i]
		value := fmt.Sprintf("secret_%d", i)
		if opts.LongValues {
			value += "_" + strings.Repeat("s", opts.ValueLength)
		}
		data[key] = value
	}

	return data
}

// TestDataOptions defines options for generating test data
type TestDataOptions struct {
	RegularVars   int
	SensitiveVars int
	LongValues    bool
	ValueLength   int
}

// DefaultTestDataOptions returns default test data options
func (f *TestFixtures) DefaultTestDataOptions() TestDataOptions {
	return TestDataOptions{
		RegularVars:   10,
		SensitiveVars: 5,
		LongValues:    false,
		ValueLength:   50,
	}
}

// BenchmarkDataOptions returns options for benchmark testing
func (f *TestFixtures) BenchmarkDataOptions() TestDataOptions {
	return TestDataOptions{
		RegularVars:   1000,
		SensitiveVars: 100,
		LongValues:    true,
		ValueLength:   200,
	}
}
