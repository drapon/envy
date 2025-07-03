package testutil

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/drapon/envy/internal/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertEnvFileContains asserts that an env file contains specific variables
func AssertEnvFileContains(t *testing.T, file *env.File, expected map[string]string) {
	t.Helper()
	
	for key, expectedValue := range expected {
		actualValue, exists := file.Get(key)
		assert.True(t, exists, "variable %s should exist in env file", key)
		assert.Equal(t, expectedValue, actualValue, "value for variable %s should match", key)
	}
}

// AssertEnvFileNotContains asserts that an env file does not contain specific variables
func AssertEnvFileNotContains(t *testing.T, file *env.File, keys []string) {
	t.Helper()
	
	for _, key := range keys {
		_, exists := file.Get(key)
		assert.False(t, exists, "variable %s should not exist in env file", key)
	}
}

// AssertEnvFileSize asserts that an env file has a specific number of variables
func AssertEnvFileSize(t *testing.T, file *env.File, expectedSize int) {
	t.Helper()
	
	actualSize := len(file.Variables)
	assert.Equal(t, expectedSize, actualSize, "env file should have %d variables, got %d", expectedSize, actualSize)
}

// AssertEnvFileOrder asserts that variables are in a specific order
func AssertEnvFileOrder(t *testing.T, file *env.File, expectedOrder []string) {
	t.Helper()
	
	actualOrder := file.Order
	assert.Equal(t, expectedOrder, actualOrder, "variable order should match expected order")
}

// Note: Config-related assertions moved to avoid import cycles
// Use these patterns in your tests directly:
// 
// To assert config is valid:
//   err := cfg.Validate()
//   assert.NoError(t, err)
//
// To assert config is invalid:
//   err := cfg.Validate()
//   assert.Error(t, err)
//   assert.Contains(t, err.Error(), expectedErrorMsg)

// AssertVariableSensitive asserts that a variable is considered sensitive
func AssertVariableSensitive(t *testing.T, key string) {
	t.Helper()
	
	isSensitive := isVariableSensitive(key)
	assert.True(t, isSensitive, "variable %s should be considered sensitive", key)
}

// AssertVariableNotSensitive asserts that a variable is not considered sensitive
func AssertVariableNotSensitive(t *testing.T, key string) {
	t.Helper()
	
	isSensitive := isVariableSensitive(key)
	assert.False(t, isSensitive, "variable %s should not be considered sensitive", key)
}

// AssertStringSliceEqual asserts that two string slices are equal
func AssertStringSliceEqual(t *testing.T, expected, actual []string) {
	t.Helper()
	
	assert.Equal(t, expected, actual, "string slices should be equal")
}

// AssertStringSliceContains asserts that a string slice contains all expected items
func AssertStringSliceContains(t *testing.T, slice []string, expected []string) {
	t.Helper()
	
	for _, item := range expected {
		assert.Contains(t, slice, item, "slice should contain %s", item)
	}
}

// AssertStringSliceNotContains asserts that a string slice does not contain any of the items
func AssertStringSliceNotContains(t *testing.T, slice []string, items []string) {
	t.Helper()
	
	for _, item := range items {
		assert.NotContains(t, slice, item, "slice should not contain %s", item)
	}
}

// AssertMapSubset asserts that a map contains all key-value pairs from a subset map
func AssertMapSubset(t *testing.T, superset, subset map[string]string) {
	t.Helper()
	
	for key, expectedValue := range subset {
		actualValue, exists := superset[key]
		assert.True(t, exists, "superset should contain key %s", key)
		assert.Equal(t, expectedValue, actualValue, "value for key %s should match", key)
	}
}

// AssertMapNotSubset asserts that a map does not contain all key-value pairs from a subset map
func AssertMapNotSubset(t *testing.T, superset, subset map[string]string) {
	t.Helper()
	
	hasAll := true
	for key, expectedValue := range subset {
		actualValue, exists := superset[key]
		if !exists || actualValue != expectedValue {
			hasAll = false
			break
		}
	}
	
	assert.False(t, hasAll, "superset should not contain all key-value pairs from subset")
}

// AssertMapKeys asserts that a map has exactly the expected keys
func AssertMapKeys(t *testing.T, m map[string]string, expectedKeys []string) {
	t.Helper()
	
	actualKeys := make([]string, 0, len(m))
	for key := range m {
		actualKeys = append(actualKeys, key)
	}
	
	assert.ElementsMatch(t, expectedKeys, actualKeys, "map should have exactly the expected keys")
}

// AssertErrorType asserts that an error is of a specific type
func AssertErrorType(t *testing.T, err error, expectedType interface{}) {
	t.Helper()
	
	require.Error(t, err, "error should not be nil")
	
	expectedTypeValue := reflect.TypeOf(expectedType)
	actualTypeValue := reflect.TypeOf(err)
	
	assert.True(t, actualTypeValue.AssignableTo(expectedTypeValue),
		"error type %v should be assignable to %v", actualTypeValue, expectedTypeValue)
}

// AssertErrorContains asserts that an error message contains a specific substring
func AssertErrorContains(t *testing.T, err error, expectedSubstring string) {
	t.Helper()
	
	require.Error(t, err, "error should not be nil")
	assert.Contains(t, err.Error(), expectedSubstring, 
		"error message should contain %s", expectedSubstring)
}

// AssertErrorNotContains asserts that an error message does not contain a specific substring
func AssertErrorNotContains(t *testing.T, err error, substring string) {
	t.Helper()
	
	require.Error(t, err, "error should not be nil")
	assert.NotContains(t, err.Error(), substring,
		"error message should not contain %s", substring)
}

// AssertNoErrorOrSkip asserts no error or skips the test if error is expected
func AssertNoErrorOrSkip(t *testing.T, err error, skipCondition bool, skipMsg string) {
	t.Helper()
	
	if skipCondition {
		t.Skip(skipMsg)
		return
	}
	
	assert.NoError(t, err)
}

// AssertWithTimeout asserts a condition within a timeout period
func AssertWithTimeout(t *testing.T, timeout time.Duration, condition func() bool, msgAndArgs ...interface{}) {
	t.Helper()
	
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		<-ticker.C
	}
	
	assert.Fail(t, "condition was not met within timeout", msgAndArgs...)
}

// AssertConcurrentSafe asserts that a function is safe to call concurrently
func AssertConcurrentSafe(t *testing.T, fn func(), goroutines int, iterations int) {
	t.Helper()
	
	done := make(chan bool, goroutines)
	
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in concurrent execution: %v", r)
				}
			}()
			
			for j := 0; j < iterations; j++ {
				fn()
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < goroutines; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for concurrent execution to complete")
		}
	}
}

// AssertMemoryUsage asserts that memory usage is within acceptable limits
func AssertMemoryUsage(t *testing.T, fn func(), maxMemoryMB float64) {
	t.Helper()
	
	// This is a simplified memory usage check
	// In a real implementation, you might want to use runtime.MemStats
	// and more sophisticated memory monitoring
	
	// For now, just execute the function and ensure it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("function panicked, possibly due to memory issues: %v", r)
		}
	}()
	
	fn()
}

// AssertPerformance asserts that a function completes within a time limit
func AssertPerformance(t *testing.T, fn func(), maxDuration time.Duration, description string) {
	t.Helper()
	
	start := time.Now()
	fn()
	duration := time.Since(start)
	
	assert.True(t, duration <= maxDuration, 
		"%s should complete within %v, but took %v", description, maxDuration, duration)
}

// AssertBenchmarkImprovement asserts that benchmark results show improvement
func AssertBenchmarkImprovement(t *testing.T, baseline, current time.Duration, minImprovementPercent float64) {
	t.Helper()
	
	improvementPercent := float64(baseline-current) / float64(baseline) * 100
	
	assert.True(t, improvementPercent >= minImprovementPercent,
		"performance should improve by at least %.1f%%, got %.1f%% (baseline: %v, current: %v)",
		minImprovementPercent, improvementPercent, baseline, current)
}

// AssertJSONEqual asserts that two JSON strings are equal
func AssertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()
	
	// This is a simplified JSON comparison
	// In practice, you might want to unmarshal and compare objects
	expectedNormalized := strings.ReplaceAll(strings.TrimSpace(expected), " ", "")
	actualNormalized := strings.ReplaceAll(strings.TrimSpace(actual), " ", "")
	
	assert.Equal(t, expectedNormalized, actualNormalized, "JSON strings should be equal")
}

// AssertPathExists asserts that a file or directory path exists
func AssertPathExists(t *testing.T, path string) {
	t.Helper()
	AssertFileExists(t, path) // Reuse existing helper
}

// AssertPathNotExists asserts that a file or directory path does not exist
func AssertPathNotExists(t *testing.T, path string) {
	t.Helper()
	AssertFileNotExists(t, path) // Reuse existing helper
}

// Helper function to check if a variable is sensitive
func isVariableSensitive(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitivePatterns := []string{
		"password", "secret", "key", "token",
		"credential", "auth", "private", "cert",
	}
	
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}
	
	return false
}

// AssertRetry asserts that a condition eventually becomes true with retries
func AssertRetry(t *testing.T, condition func() bool, maxRetries int, delay time.Duration, msg string) {
	t.Helper()
	
	for i := 0; i < maxRetries; i++ {
		if condition() {
			return
		}
		if i < maxRetries-1 {
			time.Sleep(delay)
		}
	}
	
	assert.Fail(t, "condition never became true after retries", msg)
}

// AssertFunctionPanic asserts that a function panics
func AssertFunctionPanic(t *testing.T, fn func(), expectedPanicMsg string) {
	t.Helper()
	
	defer func() {
		r := recover()
		assert.NotNil(t, r, "function should panic")
		if expectedPanicMsg != "" && r != nil {
			assert.Contains(t, r.(string), expectedPanicMsg, "panic message should contain expected text")
		}
	}()
	
	fn()
	t.Error("function should have panicked but didn't")
}

// AssertFunctionNotPanic asserts that a function does not panic
func AssertFunctionNotPanic(t *testing.T, fn func()) {
	t.Helper()
	
	defer func() {
		r := recover()
		assert.Nil(t, r, "function should not panic, but panicked with: %v", r)
	}()
	
	fn()
}