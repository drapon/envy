package retry

import (
	"context"
	"time"

	"github.com/drapon/envy/internal/errors"
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	// Operation timeout
	Operation time.Duration
	// AWS API timeout
	AWS time.Duration
	// Network timeout
	Network time.Duration
	// File operation timeout
	File time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Operation: 30 * time.Second,
		AWS:       60 * time.Second,
		Network:   30 * time.Second,
		File:      10 * time.Second,
	}
}

// WithTimeout executes a function with a timeout
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New(errors.ErrNetworkTimeout, "operation timed out").
				WithRetriable(true).
				WithDetails("timeout", timeout.String())
		}
		return ctx.Err()
	}
}

// WithAWSTimeout executes an AWS operation with appropriate timeout
func WithAWSTimeout(ctx context.Context, fn func(context.Context) error) error {
	config := DefaultTimeoutConfig()
	return WithTimeout(ctx, config.AWS, func(ctx context.Context) error {
		err := fn(ctx)
		if err != nil {
			// Check if it's a timeout error
			if ctx.Err() == context.DeadlineExceeded {
				return errors.New(errors.ErrAWSTimeout, "AWS operation timed out").
					WithCause(err).
					WithRetriable(true)
			}
		}
		return err
	})
}

// WithNetworkTimeout executes a network operation with timeout
func WithNetworkTimeout(ctx context.Context, fn func(context.Context) error) error {
	config := DefaultTimeoutConfig()
	return WithTimeout(ctx, config.Network, func(ctx context.Context) error {
		err := fn(ctx)
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			return errors.New(errors.ErrNetworkTimeout, "network operation timed out").
				WithCause(err).
				WithRetriable(true)
		}
		return err
	})
}

// WithFileTimeout executes a file operation with timeout
func WithFileTimeout(ctx context.Context, fn func(context.Context) error) error {
	config := DefaultTimeoutConfig()
	return WithTimeout(ctx, config.File, func(ctx context.Context) error {
		err := fn(ctx)
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			return errors.New(errors.ErrFileInvalid, "file operation timed out").
				WithCause(err)
		}
		return err
	})
}

// TimeoutOperation represents an operation with timeout and retry
type TimeoutOperation struct {
	Timeout time.Duration
	Retryer *Retryer
	OnRetry NotifyFunc
}

// Execute runs the operation with timeout and retry logic
func (t *TimeoutOperation) Execute(ctx context.Context, operation func(context.Context) error) error {
	// Wrap the operation with timeout
	timeoutOp := func(ctx context.Context) error {
		return WithTimeout(ctx, t.Timeout, operation)
	}

	// Execute with retry
	if t.Retryer != nil {
		return t.Retryer.DoWithNotify(ctx, timeoutOp, t.OnRetry)
	}

	// Execute without retry
	return timeoutOp(ctx)
}

// AWSOperation creates a timeout operation for AWS calls
func AWSOperation() *TimeoutOperation {
	return &TimeoutOperation{
		Timeout: DefaultTimeoutConfig().AWS,
		Retryer: New(AWSConfig()),
	}
}

// NetworkOperation creates a timeout operation for network calls
func NetworkOperation() *TimeoutOperation {
	return &TimeoutOperation{
		Timeout: DefaultTimeoutConfig().Network,
		Retryer: New(NetworkConfig()),
	}
}

// RunWithDeadline executes a function with an absolute deadline
func RunWithDeadline(deadline time.Time, fn func(context.Context) error) error {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	return fn(ctx)
}

// ContextWithTimeouts creates a context with multiple timeout options
type ContextWithTimeouts struct {
	ctx    context.Context
	cancel context.CancelFunc
	timers []*time.Timer
}

// NewContextWithTimeouts creates a new context with timeout management
func NewContextWithTimeouts(parent context.Context) *ContextWithTimeouts {
	ctx, cancel := context.WithCancel(parent)
	return &ContextWithTimeouts{
		ctx:    ctx,
		cancel: cancel,
		timers: make([]*time.Timer, 0),
	}
}

// AddTimeout adds a timeout that will cancel the context
func (c *ContextWithTimeouts) AddTimeout(duration time.Duration, onTimeout func()) {
	timer := time.AfterFunc(duration, func() {
		if onTimeout != nil {
			onTimeout()
		}
		c.cancel()
	})
	c.timers = append(c.timers, timer)
}

// Context returns the underlying context
func (c *ContextWithTimeouts) Context() context.Context {
	return c.ctx
}

// Cancel cancels the context and stops all timers
func (c *ContextWithTimeouts) Cancel() {
	c.cancel()
	for _, timer := range c.timers {
		timer.Stop()
	}
}

// Done returns when the context is done or any timeout expires
func (c *ContextWithTimeouts) Done() <-chan struct{} {
	return c.ctx.Done()
}
