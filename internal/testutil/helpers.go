package testutil

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "envy-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// TempFile creates a temporary file with content
func TempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "envy-test-*.env")
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	t.Cleanup(func() {
		os.Remove(f.Name())
	})

	return f.Name()
}

// WriteFile writes content to a file in a directory
func WriteFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

// ReadFile reads content from a file
func ReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// AssertFileExists asserts that a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.NoError(t, err, "file should exist: %s", path)
}

// AssertFileNotExists asserts that a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "file should not exist: %s", path)
}

// AssertStringContains asserts that a string contains a substring
func AssertStringContains(t *testing.T, str, substr string) {
	t.Helper()
	assert.Contains(t, str, substr)
}

// AssertStringNotContains asserts that a string does not contain a substring
func AssertStringNotContains(t *testing.T, str, substr string) {
	t.Helper()
	assert.NotContains(t, str, substr)
}

// AssertMapEqual asserts that two maps are equal
func AssertMapEqual(t *testing.T, expected, actual map[string]string) {
	t.Helper()
	assert.Equal(t, expected, actual)
}

// AssertMapContains asserts that a map contains all key-value pairs from another map
func AssertMapContains(t *testing.T, container, subset map[string]string) {
	t.Helper()
	for key, expectedValue := range subset {
		actualValue, exists := container[key]
		assert.True(t, exists, "key %s should exist in map", key)
		assert.Equal(t, expectedValue, actualValue, "value for key %s should match", key)
	}
}

// AssertEnvFileEqual asserts that two env files have the same variables
func AssertEnvFileEqual(t *testing.T, expected, actual *env.File) {
	t.Helper()
	expectedMap := expected.ToMap()
	actualMap := actual.ToMap()
	AssertMapEqual(t, expectedMap, actualMap)
}

// Note: CreateTestConfig moved to avoid import cycles
// Create test configs directly in your tests like:
//
// cfg := &config.Config{
//     Project:            "test-project",
//     DefaultEnvironment: "test",
//     AWS: config.AWSConfig{
//         Service: "parameter_store",
//         Region:  "us-east-1",
//     },
//     // ... other fields
// }

// CreateTestEnvFile creates a test env file with common variables
func CreateTestEnvFile() *env.File {
	file := env.NewFile()
	file.Set("APP_NAME", "test-app")
	file.Set("DEBUG", "true")
	file.Set("DATABASE_URL", "postgres://user:pass@localhost/db")
	file.Set("API_KEY", "secret-key-123")
	file.Set("PORT", "8080")
	return file
}

// CreateLargeTestEnvFile creates a large env file for performance testing
func CreateLargeTestEnvFile(size int) *env.File {
	file := env.NewFile()
	for i := 0; i < size; i++ {
		key := "VAR_" + strconv.Itoa(i)
		value := "value_" + strconv.Itoa(i) + "_" + strings.Repeat("x", 50)
		file.Set(key, value)
	}
	return file
}

// CreateTestEnvContent creates test .env file content
func CreateTestEnvContent() string {
	return `# Test environment file
APP_NAME=test-app
DEBUG=true
DATABASE_URL=postgres://user:pass@localhost/db
API_KEY=secret-key-123
PORT=8080

# Another comment
REDIS_URL=redis://localhost:6379
`
}

// CreateComplexEnvContent creates complex .env file content with various formats
func CreateComplexEnvContent() string {
	return `# Complex test environment file
SIMPLE_VAR=simple_value
QUOTED_VAR="quoted value"
SINGLE_QUOTED_VAR='single quoted'
EMPTY_VAR=
SPACES_VAR=  value with spaces  
MULTILINE_VAR="line1
line2
line3"

# Variables with special characters
SPECIAL_CHARS_VAR="value with !@#$%^&*()"
EQUALS_IN_VALUE="key=value inside"
HASH_IN_VALUE="value with # hash"

# Sensitive variables
PASSWORD=secret123
API_SECRET=very-secret-key
JWT_TOKEN=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9

# Environment specific
NODE_ENV=test
DEBUG_MODE=true
LOG_LEVEL=debug
`
}

// StringReader creates a string reader for testing
func StringReader(s string) io.Reader {
	return strings.NewReader(s)
}

// BytesBuffer creates a bytes buffer for testing
func BytesBuffer() *bytes.Buffer {
	return &bytes.Buffer{}
}

// CreateContext creates a test context with timeout
func CreateContext(timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// Store cancel function to be called by test cleanup
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return ctx
}

// CreateCancelContext creates a test context that can be cancelled
func CreateCancelContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// WaitWithTimeout waits for a condition with timeout
func WaitWithTimeout(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for condition")
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// AssertEventuallyTrue asserts that a condition becomes true within timeout
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, msgAndArgs ...interface{}) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			assert.Fail(t, "condition never became true within timeout", msgAndArgs...)
			return
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// CreateTestTable creates a test table structure for table-driven tests
type TestCase struct {
	Name     string
	Input    interface{}
	Expected interface{}
	Error    bool
	Setup    func(*testing.T)
	Cleanup  func(*testing.T)
}

// RunTestTable runs table-driven tests
func RunTestTable(t *testing.T, testCases []TestCase, testFunc func(*testing.T, TestCase)) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Setup != nil {
				tc.Setup(t)
			}

			if tc.Cleanup != nil {
				t.Cleanup(func() {
					tc.Cleanup(t)
				})
			}

			testFunc(t, tc)
		})
	}
}

// MockTime provides time mocking for tests
type MockTime struct {
	current time.Time
}

// NewMockTime creates a new mock time
func NewMockTime(t time.Time) *MockTime {
	return &MockTime{current: t}
}

// Now returns the current mocked time
func (m *MockTime) Now() time.Time {
	return m.current
}

// Add advances the mocked time
func (m *MockTime) Add(d time.Duration) {
	m.current = m.current.Add(d)
}

// Set sets the mocked time
func (m *MockTime) Set(t time.Time) {
	m.current = t
}

// CaptureStdout captures stdout to a buffer
func CaptureStdout(buf *bytes.Buffer) *os.File {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Start a goroutine to copy data from pipe to buffer
	done := make(chan struct{})
	go func() {
		io.Copy(buf, r)
		r.Close()
		close(done)
	}()

	// Return the original stdout and a cleanup function
	return oldStdout
}

// RestoreStdout restores the original stdout
func RestoreStdout(oldStdout *os.File) {
	// Close the writer to signal the goroutine to stop
	if os.Stdout != oldStdout {
		os.Stdout.Close()
	}
	os.Stdout = oldStdout
}

// CaptureOutput captures stdout and stderr for testing
func CaptureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	// Save original stdout and stderr
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Create pipes
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)

	// Replace stdout and stderr
	os.Stdout = stdoutW
	os.Stderr = stderrW

	// Channel to capture output
	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	// Read from pipes
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stdoutR)
		stdoutCh <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stderrR)
		stderrCh <- buf.String()
	}()

	// Execute function
	fn()

	// Close writers
	stdoutW.Close()
	stderrW.Close()

	// Restore original stdout and stderr
	os.Stdout = origStdout
	os.Stderr = origStderr

	// Get captured output
	stdout = <-stdoutCh
	stderr = <-stderrCh

	// Close readers
	stdoutR.Close()
	stderrR.Close()

	return stdout, stderr
}

// SetEnv sets environment variables for testing
func SetEnv(t *testing.T, key, value string) {
	t.Helper()
	original := os.Getenv(key)
	os.Setenv(key, value)

	t.Cleanup(func() {
		if original == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, original)
		}
	})
}

// UnsetEnv unsets environment variables for testing
func UnsetEnv(t *testing.T, key string) {
	t.Helper()
	original := os.Getenv(key)
	os.Unsetenv(key)

	t.Cleanup(func() {
		if original != "" {
			os.Setenv(key, original)
		}
	})
}

// ChangeDir changes directory for testing
func ChangeDir(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(dir)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Chdir(original)
	})
}

// CreateTestConfig creates a test configuration
func CreateTestConfig() *config.Config {
	return &config.Config{
		Project:            "test-project",
		DefaultEnvironment: "test",
		AWS: config.AWSConfig{
			Service: "parameter_store",
			Region:  "us-east-1",
			Profile: "default",
		},
		Cache: config.CacheConfig{
			Enabled:    true,
			Type:       "hybrid",
			TTL:        "1h",
			MaxSize:    "100MB",
			MaxEntries: 1000,
		},
		Memory: config.MemoryConfig{
			Enabled:           true,
			PoolEnabled:       true,
			MonitoringEnabled: true,
			StringPoolSize:    1024,
			BytePoolSize:      65536,
			MapPoolSize:       100,
			GCInterval:        30 * time.Second,
			MemoryThreshold:   104857600,
		},
		Performance: config.PerformanceConfig{
			BatchSize:        50,
			WorkerCount:      4,
			StreamingEnabled: true,
			BufferSize:       8192,
			MaxLineSize:      65536,
		},
		Environments: map[string]config.Environment{
			"test": {
				Files:             []string{".env.test"},
				Path:              "/test-project/test/",
				UseSecretsManager: false,
			},
			"dev": {
				Files:             []string{".env.dev"},
				Path:              "/test-project/dev/",
				UseSecretsManager: false,
			},
			"prod": {
				Files:             []string{".env.prod"},
				Path:              "/test-project/prod/",
				UseSecretsManager: true,
			},
		},
	}
}
