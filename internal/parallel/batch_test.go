package parallel

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drapon/envy/internal/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
)

// TestBatchProcessor tests basic batch processor functionality
func TestBatchProcessor(t *testing.T) {
	ctx := context.Background()

	processor := NewBatchProcessor(ctx, 2,
		WithBatchSize(5),
		WithBatchTimeout(100*time.Millisecond),
	)

	// Counter for processed items
	var processed atomic.Int32

	// Process function
	processFn := func(ctx context.Context, item interface{}) error {
		processed.Add(1)
		return nil
	}

	// Create items
	items := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = i
	}

	// Process items
	results, err := processor.Process(ctx, items, processFn)
	require.NoError(t, err)
	assert.Equal(t, 10, len(results))
	assert.Equal(t, int32(10), processed.Load())

	// Check results
	for _, result := range results {
		assert.NoError(t, result.Error)
	}
}

// TestBatchProcessorWithErrors tests error handling
func TestBatchProcessorWithErrors(t *testing.T) {
	ctx := context.Background()

	var errorCount atomic.Int32
	processor := NewBatchProcessor(ctx, 2,
		WithBatchSize(3),
		WithBatchErrorHandler(func(item interface{}, err error) {
			errorCount.Add(1)
		}),
	)

	// Process function that fails for even numbers
	processFn := func(ctx context.Context, item interface{}) error {
		num := item.(int)
		if num%2 == 0 {
			return fmt.Errorf("error for %d", num)
		}
		return nil
	}

	// Create items
	items := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = i
	}

	// Process items
	results, err := processor.Process(ctx, items, processFn)
	require.NoError(t, err)
	assert.Equal(t, 10, len(results))

	// Count errors
	var actualErrors int
	for _, result := range results {
		if result.Error != nil {
			actualErrors++
		}
	}
	assert.Equal(t, 5, actualErrors) // 0, 2, 4, 6, 8
	assert.Equal(t, int32(5), errorCount.Load())
}

// TestEnvVarBatchProcessor tests environment variable batch processing
func TestEnvVarBatchProcessor(t *testing.T) {
	ctx := context.Background()

	processor := NewEnvVarBatchProcessor(ctx, 2,
		WithBatchSize(5),
	)

	vars := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
		"VAR3": "value3",
		"VAR4": "value4",
		"VAR5": "value5",
	}

	var processed atomic.Int32
	processFn := func(ctx context.Context, key, value string) error {
		processed.Add(1)
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	errorMap, err := processor.ProcessEnvVars(ctx, vars, processFn)
	require.NoError(t, err)
	assert.Equal(t, 0, len(errorMap))
	assert.Equal(t, int32(5), processed.Load())
}

// TestRateLimiter tests rate limiting functionality
func TestRateLimiter(t *testing.T) {
	// Create rate limiter with 10 requests per second
	rl := NewRateLimiter(10, 5)
	defer rl.Stop()

	ctx := context.Background()
	start := time.Now()

	// Try to make 15 requests
	for i := 0; i < 15; i++ {
		err := rl.Wait(ctx)
		require.NoError(t, err)
	}

	elapsed := time.Since(start)

	// Should take at least 500ms for 15 requests at 10/sec
	// (first 5 are immediate due to burst, next 10 are rate-limited)
	assert.Greater(t, elapsed.Milliseconds(), int64(500))
}

// TestRateLimitedBatchProcessor tests rate-limited batch processing
func TestRateLimitedBatchProcessor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limit test in short mode")
	}

	ctx := context.Background()

	processor := NewRateLimitedBatchProcessor(ctx, 2, 5, 3,
		WithBatchSize(2),
	)
	defer processor.Stop()

	// Process function
	var processed atomic.Int32
	processFn := func(ctx context.Context, item interface{}) error {
		processed.Add(1)
		return nil
	}

	// Create items
	items := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = i
	}

	start := time.Now()
	results, err := processor.ProcessWithRateLimit(ctx, items, processFn)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 10, len(results))
	assert.Equal(t, int32(10), processed.Load())

	// Should be rate limited (roughly 2 seconds for 10 items at 5/sec)
	assert.Greater(t, elapsed.Milliseconds(), int64(1000))
}

// TestAWSBatchProcessor tests AWS batch processor
func TestAWSBatchProcessor(t *testing.T) {
	ctx := context.Background()

	// Test Parameter Store processor
	psProcessor := NewAWSBatchProcessor(ctx, "parameter_store", 2)
	assert.NotNil(t, psProcessor)

	// Test Secrets Manager processor
	smProcessor := NewAWSBatchProcessor(ctx, "secrets_manager", 2)
	assert.NotNil(t, smProcessor)

	// Process function
	processFn := func(ctx context.Context, item interface{}) error {
		// Simulate AWS API call
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	// Create operations
	operations := make([]interface{}, 5)
	for i := 0; i < 5; i++ {
		operations[i] = fmt.Sprintf("operation-%d", i)
	}

	// Test processing
	results, err := psProcessor.ProcessAWSOperations(ctx, operations, processFn)
	require.NoError(t, err)
	assert.Equal(t, 5, len(results))
}

// TestBatchProcessorRetry tests retry functionality
func TestBatchProcessorRetry(t *testing.T) {
	ctx := context.Background()

	retryConfig := retry.Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}

	processor := NewBatchProcessor(ctx, 2,
		WithBatchSize(2),
		WithBatchRetry(retryConfig),
	)

	// Track attempts per item
	attempts := make(map[int]int)
	var mu sync.Mutex

	// Process function that fails twice then succeeds
	processFn := func(ctx context.Context, item interface{}) error {
		num := item.(int)

		mu.Lock()
		attempts[num]++
		count := attempts[num]
		mu.Unlock()

		if count < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	// Create items
	items := make([]interface{}, 4)
	for i := 0; i < 4; i++ {
		items[i] = i
	}

	// Process items
	results, err := processor.Process(ctx, items, processFn)
	require.NoError(t, err)
	assert.Equal(t, 4, len(results))

	// Check that all items succeeded after retries
	for _, result := range results {
		assert.NoError(t, result.Error)
	}

	// Check retry counts
	for i := 0; i < 4; i++ {
		assert.Equal(t, 3, attempts[i], "item %d should have been tried 3 times", i)
	}
}

// BenchmarkBatchProcessor benchmarks batch processing performance
func BenchmarkBatchProcessor(b *testing.B) {
	ctx := context.Background()

	processor := NewBatchProcessor(ctx, 4,
		WithBatchSize(100),
	)

	// Simple process function
	processFn := func(ctx context.Context, item interface{}) error {
		// Simulate some work
		time.Sleep(time.Microsecond)
		return nil
	}

	b.ResetTimer()

	// Create items
	items := make([]interface{}, b.N)
	for i := 0; i < b.N; i++ {
		items[i] = i
	}

	// Process items
	_, err := processor.Process(ctx, items, processFn)
	if err != nil {
		b.Fatal(err)
	}
}

// BenchmarkRateLimitedProcessor benchmarks rate-limited processing
func BenchmarkRateLimitedProcessor(b *testing.B) {
	ctx := context.Background()

	processor := NewRateLimitedBatchProcessor(ctx, 4, 1000, 100,
		WithBatchSize(50),
	)
	defer processor.Stop()

	// Simple process function
	processFn := func(ctx context.Context, item interface{}) error {
		return nil
	}

	b.ResetTimer()

	// Create items
	items := make([]interface{}, b.N)
	for i := 0; i < b.N; i++ {
		items[i] = i
	}

	// Process items
	_, err := processor.ProcessWithRateLimit(ctx, items, processFn)
	if err != nil {
		b.Fatal(err)
	}
}
