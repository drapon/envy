package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	internalErrors "github.com/drapon/envy/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()
		assert.Equal(t, 3, config.MaxAttempts)
		assert.Equal(t, 1*time.Second, config.InitialDelay)
		assert.Equal(t, 30*time.Second, config.MaxDelay)
		assert.Equal(t, 2.0, config.Multiplier)
		assert.True(t, config.Jitter)
		assert.Equal(t, StrategyExponential, config.Strategy)
	})

	t.Run("AWSConfig", func(t *testing.T) {
		config := AWSConfig()
		assert.Equal(t, 5, config.MaxAttempts)
		assert.Equal(t, 500*time.Millisecond, config.InitialDelay)
		assert.Equal(t, 20*time.Second, config.MaxDelay)
	})

	t.Run("NetworkConfig", func(t *testing.T) {
		config := NetworkConfig()
		assert.Equal(t, 4, config.MaxAttempts)
		assert.Equal(t, 2*time.Second, config.InitialDelay)
		assert.Equal(t, 60*time.Second, config.MaxDelay)
	})
}

func TestRetryer(t *testing.T) {
	t.Run("SuccessOnFirstAttempt", func(t *testing.T) {
		retryer := NewWithDefaults()
		attempts := 0

		err := retryer.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("SuccessAfterRetries", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       false,
			Strategy:     StrategyExponential,
		}
		retryer := New(config)
		attempts := 0

		err := retryer.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return internalErrors.New(internalErrors.ErrNetworkTimeout, "timeout").
					WithRetriable(true)
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("NonRetriableError", func(t *testing.T) {
		retryer := NewWithDefaults()
		attempts := 0
		expectedErr := internalErrors.New(internalErrors.ErrConfigNotFound, "not found")

		err := retryer.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return expectedErr
		})

		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("MaxAttemptsExceeded", func(t *testing.T) {
		config := Config{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       false,
			Strategy:     StrategyExponential,
		}
		retryer := New(config)
		attempts := 0
		retriableErr := internalErrors.New(internalErrors.ErrNetworkTimeout, "timeout").
			WithRetriable(true)

		err := retryer.Do(context.Background(), func(ctx context.Context) error {
			attempts++
			return retriableErr
		})

		require.Error(t, err)
		assert.Equal(t, 2, attempts)
		assert.Contains(t, err.Error(), "operation failed after 2 attempts")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		config := Config{
			MaxAttempts:  5,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   2.0,
			Jitter:       false,
			Strategy:     StrategyExponential,
		}
		retryer := New(config)

		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		// Cancel context after first attempt
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := retryer.Do(ctx, func(ctx context.Context) error {
			attempts++
			return internalErrors.New(internalErrors.ErrNetworkTimeout, "timeout").
				WithRetriable(true)
		})

		require.Error(t, err)
		assert.Equal(t, 1, attempts)
		assert.Contains(t, err.Error(), "retry interrupted")
	})

	t.Run("WithNotify", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       false,
			Strategy:     StrategyExponential,
		}
		retryer := New(config)

		notifications := []struct {
			attempt int
			delay   time.Duration
		}{}

		err := retryer.DoWithNotify(
			context.Background(),
			func(ctx context.Context) error {
				return internalErrors.New(internalErrors.ErrNetworkTimeout, "timeout").
					WithRetriable(true)
			},
			func(err error, attempt int, delay time.Duration) {
				notifications = append(notifications, struct {
					attempt int
					delay   time.Duration
				}{attempt, delay})
			},
		)

		require.Error(t, err)
		assert.Len(t, notifications, 2) // 2 retries (not including final attempt)
		assert.Equal(t, 1, notifications[0].attempt)
		assert.Equal(t, 2, notifications[1].attempt)
	})
}

func TestDelayCalculation(t *testing.T) {
	t.Run("ExponentialBackoff", func(t *testing.T) {
		config := Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       false,
			Strategy:     StrategyExponential,
		}
		retryer := New(config)

		// Test delay progression
		assert.Equal(t, 100*time.Millisecond, retryer.calculateDelay(1))
		assert.Equal(t, 200*time.Millisecond, retryer.calculateDelay(2))
		assert.Equal(t, 400*time.Millisecond, retryer.calculateDelay(3))
		assert.Equal(t, 800*time.Millisecond, retryer.calculateDelay(4))
	})

	t.Run("LinearBackoff", func(t *testing.T) {
		config := Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Strategy:     StrategyLinear,
			Jitter:       false,
		}
		retryer := New(config)

		assert.Equal(t, 100*time.Millisecond, retryer.calculateDelay(1))
		assert.Equal(t, 200*time.Millisecond, retryer.calculateDelay(2))
		assert.Equal(t, 300*time.Millisecond, retryer.calculateDelay(3))
	})

	t.Run("ConstantBackoff", func(t *testing.T) {
		config := Config{
			InitialDelay: 500 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Strategy:     StrategyConstant,
			Jitter:       false,
		}
		retryer := New(config)

		assert.Equal(t, 500*time.Millisecond, retryer.calculateDelay(1))
		assert.Equal(t, 500*time.Millisecond, retryer.calculateDelay(2))
		assert.Equal(t, 500*time.Millisecond, retryer.calculateDelay(3))
	})

	t.Run("MaxDelayEnforced", func(t *testing.T) {
		config := Config{
			InitialDelay: 1 * time.Second,
			MaxDelay:     2 * time.Second,
			Multiplier:   10.0,
			Jitter:       false,
			Strategy:     StrategyExponential,
		}
		retryer := New(config)

		// Should be capped at MaxDelay
		assert.Equal(t, 2*time.Second, retryer.calculateDelay(5))
	})

	t.Run("WithJitter", func(t *testing.T) {
		config := Config{
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
			Jitter:       true,
			Strategy:     StrategyConstant,
		}
		retryer := New(config)

		// Jitter should add up to 20% variation
		delay := retryer.calculateDelay(1)
		assert.GreaterOrEqual(t, delay, 1*time.Second)
		assert.LessOrEqual(t, delay, 1200*time.Millisecond)
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("RetryableFunc", func(t *testing.T) {
		called := false
		op := RetryableFunc(func() error {
			called = true
			return nil
		})

		err := op(context.Background())
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("RetryableAWSOperation", func(t *testing.T) {
		awsErr := errors.New("ThrottlingException: Rate exceeded")
		op := RetryableAWSOperation(func() error {
			return awsErr
		})

		err := op(context.Background())
		require.Error(t, err)

		var envyErr *internalErrors.EnvyError
		assert.True(t, errors.As(err, &envyErr))
		assert.True(t, envyErr.Retriable)
	})

	t.Run("WithRetry", func(t *testing.T) {
		attempts := 0
		err := WithRetry(context.Background(), func() error {
			attempts++
			if attempts < 2 {
				return internalErrors.New(internalErrors.ErrNetworkTimeout, "timeout").
					WithRetriable(true)
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	t.Run("WithAWSRetry", func(t *testing.T) {
		attempts := 0
		err := WithAWSRetry(context.Background(), func() error {
			attempts++
			if attempts < 3 {
				return errors.New("ThrottlingException: Rate exceeded")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("WithNetworkRetry", func(t *testing.T) {
		attempts := 0
		err := WithNetworkRetry(context.Background(), func() error {
			attempts++
			if attempts < 2 {
				return errors.New("connection refused")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})
}

func TestCustomRetryPolicy(t *testing.T) {
	policy := RetryPolicy{
		ShouldRetry: func(err error, attempt int) bool {
			// Only retry on specific error and up to 2 attempts
			return err.Error() == "retry me" && attempt < 3
		},
		CalculateDelay: func(attempt int) time.Duration {
			return time.Duration(attempt*100) * time.Millisecond
		},
	}

	attempts := 0
	err := DoWithPolicy(
		context.Background(),
		func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("retry me")
			}
			return nil
		},
		policy,
		5, // max attempts
	)

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestTimeoutHandling(t *testing.T) {
	t.Run("OperationTimeout", func(t *testing.T) {
		config := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			Timeout:      50 * time.Millisecond,
		}
		retryer := New(config)

		err := retryer.Do(context.Background(), func(ctx context.Context) error {
			// Check if context is already cancelled
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				// Simulate slow operation
				return nil
			}
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "deadline exceeded")
	})
}
