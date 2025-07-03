package parallel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/retry"
	"go.uber.org/zap"
)

// BatchProcessor processes items in batches with parallel execution
type BatchProcessor struct {
	batchSize    int
	maxWorkers   int
	timeout      time.Duration
	retryConfig  retry.Config
	errorHandler func(item interface{}, err error)
	pool         *WorkerPool
}

// BatchOption is a configuration option for BatchProcessor
type BatchOption func(*BatchProcessor)

// WithBatchSize sets the batch size
func WithBatchSize(size int) BatchOption {
	return func(b *BatchProcessor) {
		if size > 0 {
			b.batchSize = size
		}
	}
}

// WithBatchTimeout sets the timeout for batch processing
func WithBatchTimeout(d time.Duration) BatchOption {
	return func(b *BatchProcessor) {
		b.timeout = d
	}
}

// WithBatchRetry sets the retry configuration
func WithBatchRetry(cfg retry.Config) BatchOption {
	return func(b *BatchProcessor) {
		b.retryConfig = cfg
	}
}

// WithBatchErrorHandler sets the error handler
func WithBatchErrorHandler(handler func(item interface{}, err error)) BatchOption {
	return func(b *BatchProcessor) {
		b.errorHandler = handler
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(ctx context.Context, maxWorkers int, opts ...BatchOption) *BatchProcessor {
	b := &BatchProcessor{
		batchSize:   100,
		maxWorkers:  maxWorkers,
		timeout:     5 * time.Minute,
		retryConfig: retry.DefaultConfig(),
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	// Create worker pool
	poolOpts := []PoolOption{
		WithMaxWorkers(maxWorkers),
		WithTimeout(b.timeout),
		WithBufferSize(b.batchSize * 2),
	}
	b.pool = NewWorkerPool(ctx, poolOpts...)

	// Default error handler
	if b.errorHandler == nil {
		b.errorHandler = func(item interface{}, err error) {
			log.Error("Batch processing error",
				zap.Any("item", item),
				zap.Error(err),
			)
		}
	}

	return b
}

// ProcessFunc is a function that processes a single item
type ProcessFunc func(ctx context.Context, item interface{}) error

// Process processes items in batches
func (b *BatchProcessor) Process(ctx context.Context, items []interface{}, fn ProcessFunc) ([]Result, error) {
	if len(items) == 0 {
		return []Result{}, nil
	}

	log.Info("Starting batch processing",
		zap.Int("total_items", len(items)),
		zap.Int("batch_size", b.batchSize),
		zap.Int("max_workers", b.maxWorkers),
	)

	// Start worker pool
	b.pool.Start()
	defer b.pool.Stop()

	// Create batches
	batches := b.createBatches(items)
	totalBatches := len(batches)

	log.Debug("Batch creation completed",
		zap.Int("total_batches", totalBatches),
	)

	// Process batches
	var allResults []Result
	var mu sync.Mutex

	for i, batch := range batches {
		batchNum := i + 1
		batchItems := batch

		// Create task for this batch
		task := NewTaskFunc(
			fmt.Sprintf("batch_%d", batchNum),
			func(ctx context.Context) error {
				results := b.processBatch(ctx, batchItems, fn)

				mu.Lock()
				allResults = append(allResults, results...)
				mu.Unlock()

				log.Debug("Batch processing completed",
					zap.Int("batch", batchNum),
					zap.Int("total_batches", totalBatches),
					zap.Int("items", len(batchItems)),
				)

				return nil
			},
			false, // Batch level retry is handled internally
		)

		// Submit batch task
		if err := b.pool.Submit(task); err != nil {
			return nil, fmt.Errorf("failed to submit batch task: %w", err)
		}
	}

	// Wait for all batches to complete
	poolResults := b.pool.Wait()

	// Check for batch-level errors
	for _, result := range poolResults {
		if result.Error != nil {
			log.Error("Batch level error",
				zap.String("batch", result.Task.Name()),
				zap.Error(result.Error),
			)
		}
	}

	log.Info("Batch processing completed",
		zap.Int("total_results", len(allResults)),
	)

	return allResults, nil
}

// processBatch processes a single batch of items
func (b *BatchProcessor) processBatch(ctx context.Context, items []interface{}, fn ProcessFunc) []Result {
	var results []Result
	var wg sync.WaitGroup
	resultChan := make(chan Result, len(items))

	for _, item := range items {
		wg.Add(1)
		go func(item interface{}) {
			defer wg.Done()

			start := time.Now()
			var err error

			// Create retry wrapper
			retryer := retry.New(b.retryConfig)
			err = retryer.Do(ctx, func(ctx context.Context) error {
				return fn(ctx, item)
			})

			duration := time.Since(start)

			// Handle error
			if err != nil {
				b.errorHandler(item, err)
			}

			// Send result
			resultChan <- Result{
				Task: &itemTask{item: item},
				Error: err,
				Time:  duration,
			}
		}(item)
	}

	// Wait for all items in batch to complete
	wg.Wait()
	close(resultChan)

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// createBatches divides items into batches
func (b *BatchProcessor) createBatches(items []interface{}) [][]interface{} {
	var batches [][]interface{}
	
	for i := 0; i < len(items); i += b.batchSize {
		end := i + b.batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// itemTask is a simple task wrapper for items
type itemTask struct {
	item interface{}
}

func (t *itemTask) Execute(ctx context.Context) error {
	return nil
}

func (t *itemTask) Name() string {
	return fmt.Sprintf("item_%v", t.item)
}

func (t *itemTask) IsRetriable() bool {
	return true
}

// EnvVarBatchProcessor is specialized for environment variable operations
type EnvVarBatchProcessor struct {
	*BatchProcessor
	awsRateLimit time.Duration
}

// NewEnvVarBatchProcessor creates a batch processor for environment variables
func NewEnvVarBatchProcessor(ctx context.Context, maxWorkers int, opts ...BatchOption) *EnvVarBatchProcessor {
	// AWS API rate limits:
	// Parameter Store: 1000 TPS for GetParameter, 100 TPS for PutParameter
	// Secrets Manager: 1500 TPS for GetSecretValue, 50 TPS for PutSecretValue
	
	processor := NewBatchProcessor(ctx, maxWorkers, opts...)
	
	return &EnvVarBatchProcessor{
		BatchProcessor: processor,
		awsRateLimit:   10 * time.Millisecond, // Conservative rate limit
	}
}

// ProcessEnvVars processes environment variables in batches
func (e *EnvVarBatchProcessor) ProcessEnvVars(
	ctx context.Context,
	vars map[string]string,
	fn func(ctx context.Context, key, value string) error,
) (map[string]error, error) {
	// Convert map to slice of items
	type envVar struct {
		Key   string
		Value string
	}

	var items []interface{}
	for k, v := range vars {
		items = append(items, &envVar{Key: k, Value: v})
	}

	// Process with rate limiting
	poolOpts := append([]PoolOption{
		WithRateLimit(e.awsRateLimit),
	})
	
	e.pool = NewWorkerPool(ctx, poolOpts...)

	// Process items
	results, err := e.Process(ctx, items, func(ctx context.Context, item interface{}) error {
		ev := item.(*envVar)
		return fn(ctx, ev.Key, ev.Value)
	})

	if err != nil {
		return nil, err
	}

	// Convert results to error map
	errorMap := make(map[string]error)
	for _, result := range results {
		if result.Error != nil {
			ev := result.Task.(*itemTask).item.(*envVar)
			errorMap[ev.Key] = result.Error
		}
	}

	return errorMap, nil
}

// RateLimitedBatchProcessor adds rate limiting to batch processing
type RateLimitedBatchProcessor struct {
	*BatchProcessor
	rateLimiter *RateLimiter
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	rate       int           // requests per second
	burst      int           // burst capacity
	tokens     chan struct{}
	refillStop chan struct{}
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     make(chan struct{}, burst),
		refillStop: make(chan struct{}),
	}

	// Fill initial tokens
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}

	// Start refill goroutine
	go rl.refill()

	return rl
}

// Wait waits for a token to become available
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.tokens:
		return nil
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.refillStop)
}

// refill refills tokens at the specified rate
func (rl *RateLimiter) refill() {
	ticker := time.NewTicker(time.Second / time.Duration(rl.rate))
	defer ticker.Stop()

	for {
		select {
		case <-rl.refillStop:
			return
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
				// Token added
			default:
				// Bucket is full
			}
		}
	}
}

// NewRateLimitedBatchProcessor creates a rate-limited batch processor
func NewRateLimitedBatchProcessor(
	ctx context.Context,
	maxWorkers int,
	rateLimit int,
	burst int,
	opts ...BatchOption,
) *RateLimitedBatchProcessor {
	processor := NewBatchProcessor(ctx, maxWorkers, opts...)
	rateLimiter := NewRateLimiter(rateLimit, burst)

	return &RateLimitedBatchProcessor{
		BatchProcessor: processor,
		rateLimiter:    rateLimiter,
	}
}

// ProcessWithRateLimit processes items with rate limiting
func (r *RateLimitedBatchProcessor) ProcessWithRateLimit(
	ctx context.Context,
	items []interface{},
	fn ProcessFunc,
) ([]Result, error) {
	// Wrap the process function with rate limiting
	rateLimitedFn := func(ctx context.Context, item interface{}) error {
		// Wait for rate limit token
		if err := r.rateLimiter.Wait(ctx); err != nil {
			return errors.Wrap(err, errors.ErrNetworkTimeout, "error during rate limit wait")
		}
		
		// Process item
		return fn(ctx, item)
	}

	return r.Process(ctx, items, rateLimitedFn)
}

// Stop stops the rate limited processor
func (r *RateLimitedBatchProcessor) Stop() {
	r.rateLimiter.Stop()
	r.pool.Stop()
}

// AWSBatchProcessor is optimized for AWS operations
type AWSBatchProcessor struct {
	*RateLimitedBatchProcessor
	service string // "parameter_store" or "secrets_manager"
}

// NewAWSBatchProcessor creates a batch processor for AWS operations
func NewAWSBatchProcessor(
	ctx context.Context,
	service string,
	maxWorkers int,
	opts ...BatchOption,
) *AWSBatchProcessor {
	// Set rate limits based on AWS service
	var rateLimit, burst int
	switch service {
	case "parameter_store":
		rateLimit = 100  // PutParameter limit
		burst = 200      // Allow some burst
	case "secrets_manager":
		rateLimit = 50   // PutSecretValue limit
		burst = 100
	default:
		rateLimit = 50   // Conservative default
		burst = 100
	}

	// Add AWS-specific retry configuration
	awsRetry := retry.AWSConfig()
	opts = append(opts, WithBatchRetry(awsRetry))

	processor := NewRateLimitedBatchProcessor(
		ctx,
		maxWorkers,
		rateLimit,
		burst,
		opts...,
	)

	return &AWSBatchProcessor{
		RateLimitedBatchProcessor: processor,
		service:                   service,
	}
}

// ProcessAWSOperations processes AWS operations with appropriate limits
func (a *AWSBatchProcessor) ProcessAWSOperations(
	ctx context.Context,
	operations []interface{},
	fn ProcessFunc,
) ([]Result, error) {
	log.Info("Starting AWS batch processing",
		zap.String("service", a.service),
		zap.Int("operations", len(operations)),
	)

	return a.ProcessWithRateLimit(ctx, operations, fn)
}