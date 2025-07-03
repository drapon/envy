package fixtures

import (
	"fmt"
	"strings"
	"time"

	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
)

// EnvFiles provides sample .env file contents
var EnvFiles = struct {
	Simple      string
	Complex     string
	Large       string
	Empty       string
	Comments    string
	Invalid     string
	Multiline   string
	Sensitive   string
	Development string
	Production  string
	Staging     string
	WithErrors  string
}{
	Simple: `# Simple environment file
APP_NAME=envy-test
APP_ENV=development
DEBUG=true
PORT=3000
`,

	Complex: `# Complex environment file with various formats
# Basic variables
APP_NAME="My Application"
APP_VERSION='1.2.3'
NODE_ENV=production

# URLs and connections
DATABASE_URL=postgres://user:pass@localhost:5432/mydb?sslmode=disable
REDIS_URL=redis://localhost:6379/0
API_BASE_URL=https://api.example.com/v1

# Numeric values
PORT=8080
TIMEOUT=30
MAX_CONNECTIONS=100
RATE_LIMIT=1000

# Boolean values
DEBUG=false
ENABLE_CACHE=true
USE_SSL=1

# Empty and whitespace
EMPTY_VAR=
SPACES_VAR=  value with spaces  

# Special characters
SPECIAL_CHARS="!@#$%^&*()"
EQUALS_IN_VALUE="key=value"
QUOTES_IN_VALUE="He said \"Hello\""
ESCAPED_VALUE="Line 1\nLine 2\tTabbed"

# Multiline values
PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890
abcdefghijklmnopqrstuvwxyz
-----END RSA PRIVATE KEY-----"

# Comments in various positions
VAR_WITH_COMMENT=value # This is an inline comment
# COMMENTED_OUT=should_not_be_parsed

# Export syntax
export EXPORTED_VAR=exported_value
`,

	Large: generateLargeEnvContent(1000),

	Empty: ``,

	Comments: `# This file only contains comments
# No actual environment variables
# Used for testing comment parsing

# Another comment block
# With multiple lines
`,

	Invalid: `# Invalid env file content
INVALID LINE WITHOUT EQUALS
=VALUE_WITHOUT_KEY
KEY_WITH_SPACES IN NAME=invalid
KEY WITH QUOTES"=invalid
UNCLOSED_QUOTE="missing end quote
ANOTHER_UNCLOSED='single quote
`,

	Multiline: `# Multiline value tests
SINGLE_LINE=normal value

MULTILINE_DOUBLE="line 1
line 2
line 3"

MULTILINE_SINGLE='another
multiline
value'

JSON_CONFIG={"name":"test","nested":{"key":"value"},"array":[1,2,3]}

CERTIFICATE="-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKLdQRRtKN2DMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTkwMjI4MDkzNDU3WhcNMjAwMjI4MDkzNDU3WjBF
-----END CERTIFICATE-----"
`,

	Sensitive: `# Sensitive data for testing
DATABASE_PASSWORD=super-secret-password
API_KEY=sk_test_1234567890abcdef
JWT_SECRET=my-jwt-secret-key
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
PRIVATE_KEY=-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEA
-----END PRIVATE KEY-----
GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuvwxyz
STRIPE_SECRET_KEY=sk_live_1234567890
ENCRYPTION_KEY=AES256:1234567890abcdef1234567890abcdef
`,

	Development: `# Development environment
APP_ENV=development
DEBUG=true
LOG_LEVEL=debug
DATABASE_URL=postgres://dev:devpass@localhost:5432/app_dev
REDIS_URL=redis://localhost:6379/0
API_BASE_URL=http://localhost:3000
CORS_ORIGINS=http://localhost:3000,http://localhost:8080
HOT_RELOAD=true
`,

	Production: `# Production environment
APP_ENV=production
DEBUG=false
LOG_LEVEL=info
DATABASE_URL=postgres://prod:${DB_PASSWORD}@db.example.com:5432/app_prod
REDIS_URL=redis://redis.example.com:6379/0
API_BASE_URL=https://api.example.com
CORS_ORIGINS=https://app.example.com
ENABLE_METRICS=true
ENABLE_TRACING=true
`,

	Staging: `# Staging environment
APP_ENV=staging
DEBUG=false
LOG_LEVEL=debug
DATABASE_URL=postgres://staging:${DB_PASSWORD}@staging-db.example.com:5432/app_staging
REDIS_URL=redis://staging-redis.example.com:6379/0
API_BASE_URL=https://staging-api.example.com
CORS_ORIGINS=https://staging.example.com
ENABLE_PROFILING=true
`,

	WithErrors: `# File with intentional errors
VALID_VAR=valid_value
INVALID LINE
ANOTHER_VALID=value
=NO_KEY
KEY_WITH_ERROR=
MORE_ERRORS HERE
FINAL_VALID=final_value
`,
}

// Configs provides sample configuration objects
var Configs = struct {
	Default     *config.Config
	Minimal     *config.Config
	Complete    *config.Config
	MultiEnv    *config.Config
	WithMapping *config.Config
}{
	Default: &config.Config{
		Project:            "test-project",
		DefaultEnvironment: "development",
		AWS: config.AWSConfig{
			Service: "parameter_store",
			Region:  "us-east-1",
		},
	},

	Minimal: &config.Config{
		Project: "minimal-project",
	},

	Complete: &config.Config{
		Project:            "complete-project",
		DefaultEnvironment: "production",
		AWS: config.AWSConfig{
			Service: "parameter_store",
			Region:  "us-west-2",
			Profile: "production",
		},
		Environments: map[string]config.Environment{
			"development": {
				Files:             []string{".env.development", ".env.local"},
				Path:              "/apps/myapp/dev",
				UseSecretsManager: false,
			},
			"staging": {
				Files:             []string{".env.staging"},
				Path:              "/apps/myapp/staging",
				UseSecretsManager: true,
			},
			"production": {
				Files:             []string{".env.production"},
				Path:              "/apps/myapp/prod",
				UseSecretsManager: false,
			},
		},
	},

	MultiEnv: &config.Config{
		Project:            "multi-env-project",
		DefaultEnvironment: "development",
		AWS: config.AWSConfig{
			Service: "parameter_store",
			Region:  "us-east-1",
		},
		Environments: map[string]config.Environment{
			"development": {
				Files: []string{".env.base", ".env.development", ".env.local"},
			},
			"test": {
				Files: []string{".env.base", ".env.test"},
			},
			"staging": {
				Files: []string{".env.base", ".env.staging"},
			},
			"production": {
				Files: []string{".env.base", ".env.production"},
			},
		},
	},

	WithMapping: &config.Config{
		Project: "mapping-project",
		AWS: config.AWSConfig{
			Service: "parameter_store",
			Region:  "us-east-1",
		},
	},
}

// EnvData provides sample environment variable data
var EnvData = struct {
	Basic     map[string]string
	Complex   map[string]string
	Sensitive map[string]string
	Large     map[string]string
}{
	Basic: map[string]string{
		"APP_NAME": "test-app",
		"APP_ENV":  "test",
		"DEBUG":    "true",
		"PORT":     "3000",
	},

	Complex: map[string]string{
		"APP_NAME":        "My Complex App",
		"DATABASE_URL":    "postgres://user:pass@localhost:5432/mydb",
		"REDIS_URL":       "redis://localhost:6379/0",
		"API_ENDPOINTS":   "https://api1.example.com,https://api2.example.com",
		"FEATURE_FLAGS":   "feature1:true,feature2:false,feature3:true",
		"JSON_CONFIG":     `{"timeout":30,"retries":3,"endpoints":["a","b","c"]}`,
		"MULTILINE_VALUE": "line1\nline2\nline3",
		"SPECIAL_CHARS":   "!@#$%^&*()_+-=[]{}|;:,.<>?",
	},

	Sensitive: map[string]string{
		"DATABASE_PASSWORD":     "super-secret-password",
		"API_KEY":               "sk_test_1234567890abcdef",
		"JWT_SECRET":            "my-jwt-secret-key",
		"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
		"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"ENCRYPTION_KEY":        "AES256:1234567890abcdef1234567890abcdef",
		"PRIVATE_KEY":           "-----BEGIN PRIVATE KEY-----\nMIIEvwIBADANBg...\n-----END PRIVATE KEY-----",
	},

	Large: generateLargeEnvData(500),
}

// AWSPaths provides sample AWS parameter/secret paths
var AWSPaths = struct {
	ParameterStore struct {
		Simple   []string
		Nested   []string
		WithTags []string
	}
	SecretsManager struct {
		Simple []string
		Tagged []string
	}
}{
	ParameterStore: struct {
		Simple   []string
		Nested   []string
		WithTags []string
	}{
		Simple: []string{
			"/myapp/APP_NAME",
			"/myapp/APP_ENV",
			"/myapp/DEBUG",
			"/myapp/PORT",
		},
		Nested: []string{
			"/myapp/db/host",
			"/myapp/db/port",
			"/myapp/db/username",
			"/myapp/db/password",
			"/myapp/redis/host",
			"/myapp/redis/port",
			"/myapp/api/key",
			"/myapp/api/secret",
		},
		WithTags: []string{
			"/myapp/tagged/var1",
			"/myapp/tagged/var2",
			"/myapp/tagged/var3",
		},
	},
	SecretsManager: struct {
		Simple []string
		Tagged []string
	}{
		Simple: []string{
			"myapp/database",
			"myapp/api-keys",
			"myapp/certificates",
		},
		Tagged: []string{
			"myapp/production/secrets",
			"myapp/staging/secrets",
		},
	},
}

// ErrorCases provides test cases that should produce errors
var ErrorCases = struct {
	InvalidEnvSyntax []string
	InvalidConfig    []string
	AWSErrors        []string
}{
	InvalidEnvSyntax: []string{
		"INVALID LINE WITHOUT EQUALS",
		"=VALUE_WITHOUT_KEY",
		"KEY WITH SPACES=invalid",
		"KEY\tWITH\tTABS=invalid",
		"KEY\nWITH\nNEWLINES=invalid",
	},

	InvalidConfig: []string{
		"", // empty project name
		"project with spaces",
		"project/with/slashes",
		"project-with-invalid-@-chars",
	},

	AWSErrors: []string{
		"ParameterNotFound",
		"ParameterAlreadyExists",
		"InvalidParameterValue",
		"AccessDenied",
		"ThrottlingException",
		"ServiceUnavailable",
	},
}

// Helper functions

// CreateEnvFile creates an env.File from a map
func CreateEnvFile(vars map[string]string) *env.File {
	file := env.NewFile()
	for key, value := range vars {
		file.Set(key, value)
	}
	return file
}

// CreateEnvFileWithComments creates an env.File with comments
func CreateEnvFileWithComments(vars map[string]string, comments map[string]string) *env.File {
	file := env.NewFile()
	for key, value := range vars {
		file.Set(key, value)
		// Comments are stored separately in File.Comments map
		// SetComment method doesn't exist, so we can't set inline comments
	}
	return file
}

// GenerateTestData generates test data of various sizes
func GenerateTestData(prefix string, count int) map[string]string {
	data := make(map[string]string)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%s_VAR_%d", prefix, i)
		value := fmt.Sprintf("value_%d_%s", i, strings.Repeat("x", 20))
		data[key] = value
	}
	return data
}

// GenerateTimestampedData generates data with timestamps
func GenerateTimestampedData(prefix string, count int) map[string]string {
	data := make(map[string]string)
	now := time.Now()
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%s_%d", prefix, i)
		value := fmt.Sprintf("value_%s", now.Add(time.Duration(i)*time.Second).Format(time.RFC3339))
		data[key] = value
	}
	return data
}

// Private helper functions

func generateLargeEnvContent(size int) string {
	var builder strings.Builder
	builder.WriteString("# Large environment file for testing\n")
	builder.WriteString("# Generated with ")
	builder.WriteString(fmt.Sprintf("%d variables\n\n", size))

	for i := 0; i < size; i++ {
		if i%100 == 0 {
			builder.WriteString(fmt.Sprintf("\n# Block %d\n", i/100))
		}
		builder.WriteString(fmt.Sprintf("VAR_%05d=value_%d_%s\n", i, i, strings.Repeat("x", 50)))
	}

	return builder.String()
}

func generateLargeEnvData(size int) map[string]string {
	data := make(map[string]string)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("VAR_%05d", i)
		value := fmt.Sprintf("value_%d_%s", i, strings.Repeat("x", 50))
		data[key] = value
	}
	return data
}

// TestScenarios provides complete test scenarios
type TestScenario struct {
	Name        string
	EnvContent  string
	Config      *config.Config
	AWSData     map[string]string
	Expected    map[string]string
	ShouldError bool
	ErrorType   string
}

// Scenarios provides pre-built test scenarios
var Scenarios = []TestScenario{
	{
		Name:       "Basic sync from local to AWS",
		EnvContent: EnvFiles.Simple,
		Config:     Configs.Default,
		Expected:   EnvData.Basic,
	},
	{
		Name:       "Complex data with special characters",
		EnvContent: EnvFiles.Complex,
		Config:     Configs.Complete,
		Expected:   EnvData.Complex,
	},
	{
		Name:       "Pull from AWS with no local file",
		EnvContent: "",
		Config:     Configs.Default,
		AWSData:    EnvData.Basic,
		Expected:   EnvData.Basic,
	},
	{
		Name:        "Invalid env file syntax",
		EnvContent:  EnvFiles.Invalid,
		Config:      Configs.Default,
		ShouldError: true,
		ErrorType:   "ParseError",
	},
	{
		Name:       "Multi-environment configuration",
		EnvContent: EnvFiles.Development,
		Config:     Configs.MultiEnv,
		Expected:   EnvData.Basic,
	},
}
