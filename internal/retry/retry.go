package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/drapon/envy/internal/errors"
)

// Strategy defines the retry strategy
type Strategy string

const (
	// StrategyExponential uses exponential backoff
	StrategyExponential Strategy = "exponential"
	// StrategyLinear uses linear backoff
	StrategyLinear Strategy = "linear"
	// StrategyConstant uses constant delay
	StrategyConstant Strategy = "constant"
)

// Config holds retry configuration
type Config struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int
	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the multiplier for exponential backoff
	Multiplier float64
	// Jitter adds randomness to the delay
	Jitter bool
	// Strategy is the retry strategy to use
	Strategy Strategy
	// Timeout is the overall timeout for all retries
	Timeout time.Duration
}

// DefaultConfig returns the default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		Strategy:     StrategyExponential,
		Timeout:      5 * time.Minute,
	}
}

// AWSConfig returns retry configuration optimized for AWS
func AWSConfig() Config {
	return Config{
		MaxAttempts:  5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     20 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		Strategy:     StrategyExponential,
		Timeout:      2 * time.Minute,
	}
}

// NetworkConfig returns retry configuration for network operations
func NetworkConfig() Config {
	return Config{
		MaxAttempts:  4,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   1.5,
		Jitter:       true,
		Strategy:     StrategyExponential,
		Timeout:      3 * time.Minute,
	}
}

// Retryer handles retry logic
type Retryer struct {
	config Config
	rand   *rand.Rand
}

// New creates a new Retryer with the given configuration
func New(config Config) *Retryer {
	return &Retryer{
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewWithDefaults creates a new Retryer with default configuration
func NewWithDefaults() *Retryer {
	return New(DefaultConfig())
}

// Operation represents a retriable operation
type Operation func(ctx context.Context) error

// Do executes the operation with retry logic
func (r *Retryer) Do(ctx context.Context, operation Operation) error {
	return r.DoWithNotify(ctx, operation, nil)
}

// NotifyFunc is called before each retry attempt
type NotifyFunc func(err error, attempt int, delay time.Duration)

// DoWithNotify executes the operation with retry logic and notifications
func (r *Retryer) DoWithNotify(ctx context.Context, operation Operation, notify NotifyFunc) error {
	// Create a context with timeout if specified
	if r.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.Timeout)
		defer cancel()
	}

	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, errors.ErrTimeout, "operation canceled")
		}

		// Execute the operation
		err := operation(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if the error is retriable
		if !errors.IsRetriable(err) {
			return err
		}

		// Don't retry on the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate delay
		delay := r.calculateDelay(attempt)

		// Notify about retry
		if notify != nil {
			notify(err, attempt, delay)
		}

		// Wait before retrying
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), errors.ErrTimeout, "retry interrupted")
		}
	}

	// All retries exhausted
	return errors.Wrap(lastErr, errors.GetErrorCode(lastErr),
		fmt.Sprintf("operation failed after %d attempts", r.config.MaxAttempts))
}

// calculateDelay calculates the delay for the given attempt
func (r *Retryer) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch r.config.Strategy {
	case StrategyExponential:
		delay = r.exponentialDelay(attempt)
	case StrategyLinear:
		delay = r.linearDelay(attempt)
	case StrategyConstant:
		delay = r.config.InitialDelay
	default:
		delay = r.exponentialDelay(attempt)
	}

	// Apply jitter if enabled
	if r.config.Jitter {
		delay = r.addJitter(delay)
	}

	// Ensure delay doesn't exceed max delay
	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	return delay
}

// exponentialDelay calculates exponential backoff delay
func (r *Retryer) exponentialDelay(attempt int) time.Duration {
	// delay = initialDelay * (multiplier ^ (attempt - 1))
	multiplier := math.Pow(r.config.Multiplier, float64(attempt-1))
	return time.Duration(float64(r.config.InitialDelay) * multiplier)
}

// linearDelay calculates linear backoff delay
func (r *Retryer) linearDelay(attempt int) time.Duration {
	// delay = initialDelay * attempt
	return r.config.InitialDelay * time.Duration(attempt)
}

// addJitter adds random jitter to the delay
func (r *Retryer) addJitter(delay time.Duration) time.Duration {
	// Add up to 20% jitter
	jitter := time.Duration(r.rand.Int63n(int64(delay) / 5))
	return delay + jitter
}

// RetryableFunc wraps a function to make it retriable
func RetryableFunc(fn func() error) Operation {
	return func(ctx context.Context) error {
		return fn()
	}
}

// RetryableAWSOperation creates a retriable operation for AWS calls
func RetryableAWSOperation(fn func() error) Operation {
	return func(ctx context.Context) error {
		err := fn()
		if err != nil {
			// Wrap AWS errors to ensure proper retry logic
			return errors.WrapAWSError(err, "aws_operation", "")
		}
		return nil
	}
}

// WithRetry is a convenience function for simple retry operations
func WithRetry(ctx context.Context, fn func() error) error {
	retryer := NewWithDefaults()
	return retryer.Do(ctx, RetryableFunc(fn))
}

// WithAWSRetry is a convenience function for AWS operations
func WithAWSRetry(ctx context.Context, fn func() error) error {
	retryer := New(AWSConfig())
	return retryer.Do(ctx, RetryableAWSOperation(fn))
}

// WithNetworkRetry is a convenience function for network operations
func WithNetworkRetry(ctx context.Context, fn func() error) error {
	retryer := New(NetworkConfig())
	return retryer.Do(ctx, func(ctx context.Context) error {
		err := fn()
		if err != nil && errors.IsNetworkError(err) {
			// Ensure network errors are wrapped properly
			return errors.WrapNetworkError(err)
		}
		return err
	})
}

// RetryPolicy defines a custom retry policy
type RetryPolicy struct {
	// ShouldRetry determines if an error should be retried
	ShouldRetry func(err error, attempt int) bool
	// CalculateDelay calculates the delay for a given attempt
	CalculateDelay func(attempt int) time.Duration
}

// DoWithPolicy executes an operation with a custom retry policy
func DoWithPolicy(ctx context.Context, operation Operation, policy RetryPolicy, maxAttempts int) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check context
		if err := ctx.Err(); err != nil {
			return err
		}

		// Execute operation
		err := operation(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check custom retry policy
		if !policy.ShouldRetry(err, attempt) {
			return err
		}

		// Don't retry on last attempt
		if attempt == maxAttempts {
			break
		}

		// Calculate delay
		delay := policy.CalculateDelay(attempt)

		// Wait
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}
