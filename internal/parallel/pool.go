package parallel

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"go.uber.org/zap"
)

// Task represents a unit of work to be executed
type Task interface {
	// Execute performs the task
	Execute(ctx context.Context) error
	// Name returns a descriptive name for the task
	Name() string
	// IsRetriable returns whether the task can be retried on failure
	IsRetriable() bool
}

// TaskFunc is a function adapter for Task interface
type TaskFunc struct {
	name      string
	fn        func(ctx context.Context) error
	retriable bool
}

// NewTaskFunc creates a new TaskFunc
func NewTaskFunc(name string, fn func(ctx context.Context) error, retriable bool) *TaskFunc {
	return &TaskFunc{
		name:      name,
		fn:        fn,
		retriable: retriable,
	}
}

// Execute implements Task interface
func (t *TaskFunc) Execute(ctx context.Context) error {
	return t.fn(ctx)
}

// Name implements Task interface
func (t *TaskFunc) Name() string {
	return t.name
}

// IsRetriable implements Task interface
func (t *TaskFunc) IsRetriable() bool {
	return t.retriable
}

// Result represents the result of a task execution
type Result struct {
	Task  Task
	Error error
	Time  time.Duration
}

// WorkerPool manages a pool of workers for parallel task execution
type WorkerPool struct {
	// Configuration
	maxWorkers   int
	bufferSize   int
	timeout      time.Duration
	rateLimit    time.Duration
	errorHandler func(task Task, err error)

	// Internal state
	tasks       chan Task
	results     chan Result
	done        chan struct{}
	wg          sync.WaitGroup
	rateLimiter <-chan time.Time
	
	// Metrics
	processed    atomic.Int64
	failed       atomic.Int64
	activeWorkers atomic.Int32
	
	// Context
	ctx    context.Context
	cancel context.CancelFunc
}

// PoolOption is a configuration option for WorkerPool
type PoolOption func(*WorkerPool)

// WithMaxWorkers sets the maximum number of workers
func WithMaxWorkers(n int) PoolOption {
	return func(p *WorkerPool) {
		if n > 0 {
			p.maxWorkers = n
		}
	}
}

// WithBufferSize sets the task buffer size
func WithBufferSize(n int) PoolOption {
	return func(p *WorkerPool) {
		if n > 0 {
			p.bufferSize = n
		}
	}
}

// WithTimeout sets the timeout for each task
func WithTimeout(d time.Duration) PoolOption {
	return func(p *WorkerPool) {
		p.timeout = d
	}
}

// WithRateLimit sets the rate limit between tasks
func WithRateLimit(d time.Duration) PoolOption {
	return func(p *WorkerPool) {
		p.rateLimit = d
	}
}

// WithErrorHandler sets a custom error handler
func WithErrorHandler(handler func(task Task, err error)) PoolOption {
	return func(p *WorkerPool) {
		p.errorHandler = handler
	}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(ctx context.Context, opts ...PoolOption) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)
	
	p := &WorkerPool{
		maxWorkers: runtime.NumCPU(),
		bufferSize: 100,
		timeout:    30 * time.Second,
		rateLimit:  0,
		ctx:        poolCtx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(p)
	}

	// Initialize channels
	p.tasks = make(chan Task, p.bufferSize)
	p.results = make(chan Result, p.bufferSize)

	// Set up rate limiter if needed
	if p.rateLimit > 0 {
		p.rateLimiter = time.Tick(p.rateLimit)
	}

	// Default error handler
	if p.errorHandler == nil {
		p.errorHandler = func(task Task, err error) {
			log.Error("タスク実行エラー",
				zap.String("task", task.Name()),
				zap.Error(err),
			)
		}
	}

	log.Info("ワーカープール初期化",
		zap.Int("max_workers", p.maxWorkers),
		zap.Int("buffer_size", p.bufferSize),
		zap.Duration("timeout", p.timeout),
		zap.Duration("rate_limit", p.rateLimit),
	)

	return p
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	log.Debug("ワーカープール開始")
	
	// Start workers
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Start result collector
	go p.collectResults()
}

// Submit submits a task to the worker pool
func (p *WorkerPool) Submit(task Task) error {
	select {
	case <-p.ctx.Done():
		return errors.New(errors.ErrInternal, "ワーカープールが停止しています")
	case p.tasks <- task:
		log.Debug("タスク送信",
			zap.String("task", task.Name()),
		)
		return nil
	}
}

// SubmitBatch submits multiple tasks to the worker pool
func (p *WorkerPool) SubmitBatch(tasks []Task) error {
	for _, task := range tasks {
		if err := p.Submit(task); err != nil {
			return fmt.Errorf("タスク送信エラー: %w", err)
		}
	}
	return nil
}

// Wait waits for all tasks to complete and returns results
func (p *WorkerPool) Wait() []Result {
	// Close task channel to signal no more tasks
	close(p.tasks)
	
	// Wait for all workers to finish
	p.wg.Wait()
	
	// Close results channel
	close(p.results)
	
	// Collect all results
	var results []Result
	for result := range p.results {
		results = append(results, result)
	}

	log.Info("ワーカープール完了",
		zap.Int64("processed", p.processed.Load()),
		zap.Int64("failed", p.failed.Load()),
		zap.Int("results", len(results)),
	)

	return results
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	log.Debug("ワーカープール停止中")
	p.cancel()
	close(p.done)
}

// GetMetrics returns current pool metrics
func (p *WorkerPool) GetMetrics() (processed, failed int64, activeWorkers int32) {
	return p.processed.Load(), p.failed.Load(), p.activeWorkers.Load()
}

// worker is the main worker goroutine
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	log.Debug("ワーカー開始", zap.Int("worker_id", id))

	for {
		select {
		case <-p.ctx.Done():
			log.Debug("ワーカー停止", zap.Int("worker_id", id))
			return
		case task, ok := <-p.tasks:
			if !ok {
				log.Debug("タスクチャネルクローズ", zap.Int("worker_id", id))
				return
			}

			// Apply rate limit if configured
			if p.rateLimiter != nil {
				<-p.rateLimiter
			}

			// Execute task
			p.executeTask(task)
		}
	}
}

// executeTask executes a single task with timeout and error handling
func (p *WorkerPool) executeTask(task Task) {
	start := time.Now()
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()

	// Execute task
	err := task.Execute(ctx)
	duration := time.Since(start)

	// Update metrics
	p.processed.Add(1)
	if err != nil {
		p.failed.Add(1)
		p.errorHandler(task, err)
	}

	// Send result
	select {
	case p.results <- Result{
		Task:  task,
		Error: err,
		Time:  duration,
	}:
	case <-p.ctx.Done():
		return
	}

	log.Debug("タスク完了",
		zap.String("task", task.Name()),
		zap.Duration("duration", duration),
		zap.Bool("success", err == nil),
	)
}

// collectResults collects results in the background
func (p *WorkerPool) collectResults() {
	<-p.done
}

// DynamicWorkerPool is a worker pool that can adjust its size dynamically
type DynamicWorkerPool struct {
	*WorkerPool
	minWorkers     int
	maxWorkers     int
	scaleUpThreshold   float64
	scaleDownThreshold float64
	scalingInterval    time.Duration
	mu                 sync.Mutex
}

// NewDynamicWorkerPool creates a new dynamic worker pool
func NewDynamicWorkerPool(ctx context.Context, minWorkers, maxWorkers int, opts ...PoolOption) *DynamicWorkerPool {
	pool := NewWorkerPool(ctx, opts...)
	pool.maxWorkers = minWorkers // Start with minimum
	
	return &DynamicWorkerPool{
		WorkerPool:         pool,
		minWorkers:         minWorkers,
		maxWorkers:         maxWorkers,
		scaleUpThreshold:   0.8,
		scaleDownThreshold: 0.2,
		scalingInterval:    10 * time.Second,
	}
}

// Start starts the dynamic worker pool with auto-scaling
func (d *DynamicWorkerPool) Start() {
	d.WorkerPool.Start()
	go d.autoScale()
}

// autoScale monitors and adjusts the worker count
func (d *DynamicWorkerPool) autoScale() {
	ticker := time.NewTicker(d.scalingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.adjustWorkerCount()
		}
	}
}

// adjustWorkerCount adjusts the number of workers based on load
func (d *DynamicWorkerPool) adjustWorkerCount() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Calculate current load
	queueSize := len(d.tasks)
	activeWorkers := int(d.activeWorkers.Load())
	currentWorkers := d.maxWorkers
	
	// Calculate load ratio
	loadRatio := float64(queueSize) / float64(d.bufferSize)

	log.Debug("動的スケーリング評価",
		zap.Float64("load_ratio", loadRatio),
		zap.Int("queue_size", queueSize),
		zap.Int("active_workers", activeWorkers),
		zap.Int("current_workers", currentWorkers),
	)

	// Scale up if needed
	if loadRatio > d.scaleUpThreshold && currentWorkers < d.maxWorkers {
		newWorkers := min(currentWorkers*2, d.maxWorkers)
		for i := currentWorkers; i < newWorkers; i++ {
			d.wg.Add(1)
			go d.worker(i)
		}
		d.WorkerPool.maxWorkers = newWorkers
		log.Info("ワーカー数増加",
			zap.Int("from", currentWorkers),
			zap.Int("to", newWorkers),
		)
	}

	// Scale down if needed
	if loadRatio < d.scaleDownThreshold && currentWorkers > d.minWorkers {
		// Note: In a real implementation, we would need a way to signal
		// specific workers to stop. This is a simplified version.
		log.Info("ワーカー数削減条件を満たしましたが、実装は簡略化されています",
			zap.Int("current", currentWorkers),
			zap.Int("min", d.minWorkers),
		)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}