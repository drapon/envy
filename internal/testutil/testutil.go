package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestHelper provides common test utilities
type TestHelper struct {
	t       *testing.T
	tempDir string
	cleanup []func()
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	t.Helper()
	return &TestHelper{
		t:       t,
		cleanup: []func(){},
	}
}

// Cleanup runs all cleanup functions
func (h *TestHelper) Cleanup() {
	for i := len(h.cleanup) - 1; i >= 0; i-- {
		h.cleanup[i]()
	}
}

// AddCleanup adds a cleanup function
func (h *TestHelper) AddCleanup(fn func()) {
	h.cleanup = append(h.cleanup, fn)
}

// TempDir returns the test temporary directory, creating it if needed
func (h *TestHelper) TempDir() string {
	if h.tempDir == "" {
		dir, err := os.MkdirTemp("", "envy-test-*")
		require.NoError(h.t, err)
		h.tempDir = dir
		h.AddCleanup(func() {
			os.RemoveAll(dir)
		})
	}
	return h.tempDir
}

// CreateTempFile creates a temporary file with the given content
func (h *TestHelper) CreateTempFile(name, content string) string {
	h.t.Helper()
	path := filepath.Join(h.TempDir(), name)
	dir := filepath.Dir(path)
	
	// Create directory if needed
	if dir != h.TempDir() {
		err := os.MkdirAll(dir, 0755)
		require.NoError(h.t, err)
	}
	
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(h.t, err)
	
	return path
}

// CreateTempEnvFile creates a temporary .env file
func (h *TestHelper) CreateTempEnvFile(name string, vars map[string]string) string {
	h.t.Helper()
	
	var content strings.Builder
	for key, value := range vars {
		content.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}
	
	return h.CreateTempFile(name, content.String())
}

// SetupEnvVars sets up environment variables for testing
func (h *TestHelper) SetupEnvVars(vars map[string]string) {
	h.t.Helper()
	
	// Save original values
	original := make(map[string]string)
	for key := range vars {
		if val, exists := os.LookupEnv(key); exists {
			original[key] = val
		}
	}
	
	// Set new values
	for key, value := range vars {
		os.Setenv(key, value)
	}
	
	// Add cleanup
	h.AddCleanup(func() {
		for key := range vars {
			if val, exists := original[key]; exists {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	})
}

// TestEnvBuilder builds test environment configurations
type TestEnvBuilder struct {
	vars      map[string]string
	files     map[string]string
	awsConfig map[string]interface{}
}

// NewTestEnvBuilder creates a new test environment builder
func NewTestEnvBuilder() *TestEnvBuilder {
	return &TestEnvBuilder{
		vars:      make(map[string]string),
		files:     make(map[string]string),
		awsConfig: make(map[string]interface{}),
	}
}

// WithVar adds an environment variable
func (b *TestEnvBuilder) WithVar(key, value string) *TestEnvBuilder {
	b.vars[key] = value
	return b
}

// WithVars adds multiple environment variables
func (b *TestEnvBuilder) WithVars(vars map[string]string) *TestEnvBuilder {
	for k, v := range vars {
		b.vars[k] = v
	}
	return b
}

// WithFile adds a file to be created
func (b *TestEnvBuilder) WithFile(path, content string) *TestEnvBuilder {
	b.files[path] = content
	return b
}

// WithAWSConfig adds AWS configuration
func (b *TestEnvBuilder) WithAWSConfig(key string, value interface{}) *TestEnvBuilder {
	b.awsConfig[key] = value
	return b
}

// Build creates the test environment
func (b *TestEnvBuilder) Build(h *TestHelper) {
	// Set environment variables
	if len(b.vars) > 0 {
		h.SetupEnvVars(b.vars)
	}
	
	// Create files
	for path, content := range b.files {
		h.CreateTempFile(path, content)
	}
}

// TimeHelper provides time-related test utilities
type TimeHelper struct {
	now      time.Time
	mocked   bool
	original func() time.Time
}

// NewTimeHelper creates a new time helper
func NewTimeHelper() *TimeHelper {
	return &TimeHelper{
		now:    time.Now(),
		mocked: false,
	}
}

// Mock sets a fixed time for testing
func (th *TimeHelper) Mock(t time.Time) {
	th.now = t
	th.mocked = true
}

// Now returns the current time (mocked or real)
func (th *TimeHelper) Now() time.Time {
	if th.mocked {
		return th.now
	}
	return time.Now()
}

// Advance advances the mocked time
func (th *TimeHelper) Advance(d time.Duration) {
	if th.mocked {
		th.now = th.now.Add(d)
	}
}

// Reset resets the time helper
func (th *TimeHelper) Reset() {
	th.mocked = false
	th.now = time.Now()
}

// AssertHelper provides enhanced assertions
type AssertHelper struct {
	t *testing.T
}

// NewAssertHelper creates a new assert helper
func NewAssertHelper(t *testing.T) *AssertHelper {
	return &AssertHelper{t: t}
}

// FilePermissions asserts file has specific permissions
func (a *AssertHelper) FilePermissions(path string, expected os.FileMode) {
	a.t.Helper()
	info, err := os.Stat(path)
	require.NoError(a.t, err)
	require.Equal(a.t, expected, info.Mode().Perm(), "file %s should have permissions %v", path, expected)
}

// DirEmpty asserts directory is empty
func (a *AssertHelper) DirEmpty(path string) {
	a.t.Helper()
	entries, err := os.ReadDir(path)
	require.NoError(a.t, err)
	require.Empty(a.t, entries, "directory %s should be empty", path)
}

// DirNotEmpty asserts directory is not empty
func (a *AssertHelper) DirNotEmpty(path string) {
	a.t.Helper()
	entries, err := os.ReadDir(path)
	require.NoError(a.t, err)
	require.NotEmpty(a.t, entries, "directory %s should not be empty", path)
}

// EnvVarSet asserts environment variable is set
func (a *AssertHelper) EnvVarSet(key string) {
	a.t.Helper()
	_, exists := os.LookupEnv(key)
	require.True(a.t, exists, "environment variable %s should be set", key)
}

// EnvVarNotSet asserts environment variable is not set
func (a *AssertHelper) EnvVarNotSet(key string) {
	a.t.Helper()
	_, exists := os.LookupEnv(key)
	require.False(a.t, exists, "environment variable %s should not be set", key)
}

// EnvVarEquals asserts environment variable has specific value
func (a *AssertHelper) EnvVarEquals(key, expected string) {
	a.t.Helper()
	actual, exists := os.LookupEnv(key)
	require.True(a.t, exists, "environment variable %s should be set", key)
	require.Equal(a.t, expected, actual, "environment variable %s should equal %s", key, expected)
}

// Parallel marks test as parallel-safe and returns helper
func Parallel(t *testing.T) *TestHelper {
	t.Parallel()
	return NewTestHelper(t)
}

// SkipIfShort skips test if -short flag is set
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// SkipIfNoAWS skips test if AWS credentials are not available
func SkipIfNoAWS(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" && os.Getenv("AWS_PROFILE") == "" {
		t.Skip("Skipping test: AWS credentials not available")
	}
}

// SkipIfNoDocker skips test if Docker is not available
func SkipIfNoDocker(t *testing.T) {
	if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
		t.Skip("Skipping test: Docker not available")
	}
}

// RequireEnvVar requires an environment variable to be set
func RequireEnvVar(t *testing.T, key string) string {
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("Required environment variable %s is not set", key)
	}
	return value
}

// MustParseTime parses time string or fails test
func MustParseTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.Parse(layout, value)
	require.NoError(t, err, "failed to parse time: %s", value)
	return tm
}

// MustParseDuration parses duration string or fails test
func MustParseDuration(t *testing.T, s string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	require.NoError(t, err, "failed to parse duration: %s", s)
	return d
}

// GoldenFile manages golden file testing
type GoldenFile struct {
	t      *testing.T
	update bool
	dir    string
}

// NewGoldenFile creates a new golden file helper
func NewGoldenFile(t *testing.T, dir string) *GoldenFile {
	return &GoldenFile{
		t:      t,
		update: os.Getenv("UPDATE_GOLDEN") == "true",
		dir:    dir,
	}
}

// Assert compares actual output with golden file
func (g *GoldenFile) Assert(name string, actual []byte) {
	g.t.Helper()
	
	goldenPath := filepath.Join(g.dir, name+".golden")
	
	if g.update {
		err := os.MkdirAll(g.dir, 0755)
		require.NoError(g.t, err)
		err = os.WriteFile(goldenPath, actual, 0644)
		require.NoError(g.t, err)
		g.t.Logf("Updated golden file: %s", goldenPath)
		return
	}
	
	expected, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		g.t.Fatalf("Golden file does not exist: %s (run with UPDATE_GOLDEN=true to create)", goldenPath)
	}
	require.NoError(g.t, err)
	
	require.Equal(g.t, string(expected), string(actual), "output should match golden file")
}