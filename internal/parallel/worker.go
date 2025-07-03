package parallel

import (
	"context"
	"sync"
	"time"

	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"github.com/drapon/envy/internal/retry"
	"go.uber.org/zap"
)

// Worker represents a configurable worker for parallel task execution
type Worker struct {
	id              int
	taskQueue       chan Task
	resultQueue     chan Result
	retryConfig     *retry.Config
	errorHandler    func(task Task, err error)
	beforeExecute   func(task Task)
	afterExecute    func(task Task, result Result)
	mu              sync.RWMutex
	executing       Task
	startTime       time.Time
	totalProcessed  int64
	totalFailed     int64
}

// WorkerConfig holds configuration for a worker
type WorkerConfig struct {
	RetryConfig   *retry.Config
	ErrorHandler  func(task Task, err error)
	BeforeExecute func(task Task)
	AfterExecute  func(task Task, result Result)
}

// NewWorker creates a new worker
func NewWorker(id int, taskQueue chan Task, resultQueue chan Result, config *WorkerConfig) *Worker {
	w := &Worker{
		id:          id,
		taskQueue:   taskQueue,
		resultQueue: resultQueue,
	}

	if config != nil {
		w.retryConfig = config.RetryConfig
		w.errorHandler = config.ErrorHandler
		w.beforeExecute = config.BeforeExecute
		w.afterExecute = config.AfterExecute
	}

	// Set default retry config if not provided
	if w.retryConfig == nil {
		w.retryConfig = &retry.Config{
			MaxAttempts:     3,
			InitialDelay:    time.Second,
			MaxDelay:        30 * time.Second,
			Multiplier:      2,
			Jitter:          true,
		}
	}

	// Set default error handler if not provided
	if w.errorHandler == nil {
		w.errorHandler = func(task Task, err error) {
			log.Error("ワーカータスクエラー",
				zap.Int("worker_id", id),
				zap.String("task", task.Name()),
				zap.Error(err),
			)
		}
	}

	return w
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Debug("ワーカー開始",
		zap.Int("worker_id", w.id),
	)

	for {
		select {
		case <-ctx.Done():
			log.Debug("ワーカー停止",
				zap.Int("worker_id", w.id),
				zap.Int64("total_processed", w.totalProcessed),
				zap.Int64("total_failed", w.totalFailed),
			)
			return

		case task, ok := <-w.taskQueue:
			if !ok {
				log.Debug("タスクキューがクローズされました",
					zap.Int("worker_id", w.id),
				)
				return
			}

			w.executeTask(ctx, task)
		}
	}
}

// executeTask executes a single task with retry logic
func (w *Worker) executeTask(ctx context.Context, task Task) {
	w.mu.Lock()
	w.executing = task
	w.startTime = time.Now()
	w.mu.Unlock()

	// Call before execute hook
	if w.beforeExecute != nil {
		w.beforeExecute(task)
	}

	log.Debug("タスク実行開始",
		zap.Int("worker_id", w.id),
		zap.String("task", task.Name()),
	)

	var result Result
	result.Task = task

	// Execute task with retry if it's retriable
	if task.IsRetriable() && w.retryConfig != nil {
		retryer := retry.New(*w.retryConfig)
		err := retryer.Do(ctx, func(ctx context.Context) error {
			return task.Execute(ctx)
		})
		result.Error = err
	} else {
		result.Error = task.Execute(ctx)
	}

	result.Time = time.Since(w.startTime)

	// Update statistics
	w.mu.Lock()
	w.totalProcessed++
	if result.Error != nil {
		w.totalFailed++
		if w.errorHandler != nil {
			w.errorHandler(task, result.Error)
		}
	}
	w.executing = nil
	w.mu.Unlock()

	// Send result
	select {
	case w.resultQueue <- result:
		log.Debug("タスク完了",
			zap.Int("worker_id", w.id),
			zap.String("task", task.Name()),
			zap.Duration("duration", result.Time),
			zap.Bool("success", result.Error == nil),
		)
	case <-ctx.Done():
		log.Warn("結果送信がキャンセルされました",
			zap.Int("worker_id", w.id),
			zap.String("task", task.Name()),
		)
		return
	}

	// Call after execute hook
	if w.afterExecute != nil {
		w.afterExecute(task, result)
	}
}

// GetStatus returns the current status of the worker
func (w *Worker) GetStatus() WorkerStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	status := WorkerStatus{
		ID:             w.id,
		TotalProcessed: w.totalProcessed,
		TotalFailed:    w.totalFailed,
	}

	if w.executing != nil {
		status.IsExecuting = true
		status.CurrentTask = w.executing.Name()
		status.ExecutionTime = time.Since(w.startTime)
	}

	return status
}

// WorkerStatus represents the current status of a worker
type WorkerStatus struct {
	ID             int
	IsExecuting    bool
	CurrentTask    string
	ExecutionTime  time.Duration
	TotalProcessed int64
	TotalFailed    int64
}

// WorkerManager manages multiple workers with advanced features
type WorkerManager struct {
	workers         []*Worker
	taskQueue       chan Task
	resultQueue     chan Result
	config          *WorkerConfig
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex
	isRunning       bool
	maxQueueSize    int
	queueTimeout    time.Duration
}

// ManagerOption is a configuration option for WorkerManager
type ManagerOption func(*WorkerManager)

// WithWorkerConfig sets the worker configuration
func WithWorkerConfig(config *WorkerConfig) ManagerOption {
	return func(m *WorkerManager) {
		m.config = config
	}
}

// WithMaxQueueSize sets the maximum queue size
func WithMaxQueueSize(size int) ManagerOption {
	return func(m *WorkerManager) {
		m.maxQueueSize = size
	}
}

// WithQueueTimeout sets the queue timeout
func WithQueueTimeout(timeout time.Duration) ManagerOption {
	return func(m *WorkerManager) {
		m.queueTimeout = timeout
	}
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(ctx context.Context, numWorkers int, opts ...ManagerOption) *WorkerManager {
	managerCtx, cancel := context.WithCancel(ctx)

	m := &WorkerManager{
		workers:      make([]*Worker, 0, numWorkers),
		taskQueue:    make(chan Task, 1000), // Default buffer size
		resultQueue:  make(chan Result, 1000),
		ctx:          managerCtx,
		cancel:       cancel,
		maxQueueSize: 10000,
		queueTimeout: 30 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	// Update queue size based on configuration
	m.taskQueue = make(chan Task, m.maxQueueSize)
	m.resultQueue = make(chan Result, m.maxQueueSize)

	// Create workers
	for i := 0; i < numWorkers; i++ {
		worker := NewWorker(i, m.taskQueue, m.resultQueue, m.config)
		m.workers = append(m.workers, worker)
	}

	log.Info("ワーカーマネージャー初期化",
		zap.Int("num_workers", numWorkers),
		zap.Int("max_queue_size", m.maxQueueSize),
		zap.Duration("queue_timeout", m.queueTimeout),
	)

	return m
}

// Start starts all workers
func (m *WorkerManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return errors.New(errors.ErrInternal, "ワーカーマネージャーは既に実行中です")
	}

	log.Info("ワーカーマネージャー開始",
		zap.Int("num_workers", len(m.workers)),
	)

	for _, worker := range m.workers {
		m.wg.Add(1)
		go worker.Start(m.ctx, &m.wg)
	}

	m.isRunning = true
	return nil
}

// Submit submits a task to the worker manager
func (m *WorkerManager) Submit(task Task) error {
	m.mu.RLock()
	if !m.isRunning {
		m.mu.RUnlock()
		return errors.New(errors.ErrInternal, "ワーカーマネージャーが実行されていません")
	}
	m.mu.RUnlock()

	// Try to submit with timeout
	ctx, cancel := context.WithTimeout(m.ctx, m.queueTimeout)
	defer cancel()

	select {
	case m.taskQueue <- task:
		return nil
	case <-ctx.Done():
		return errors.New(errors.ErrTimeout, "タスクキューがタイムアウトしました")
	}
}

// SubmitBatch submits multiple tasks
func (m *WorkerManager) SubmitBatch(tasks []Task) error {
	for _, task := range tasks {
		if err := m.Submit(task); err != nil {
			return errors.Wrapf(err, "タスク '%s' の送信に失敗しました", task.Name())
		}
	}
	return nil
}

// Stop stops all workers and waits for completion
func (m *WorkerManager) Stop() []Result {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return nil
	}
	m.isRunning = false
	m.mu.Unlock()

	log.Info("ワーカーマネージャー停止中")

	// Close task queue to signal workers to stop
	close(m.taskQueue)

	// Wait for all workers to complete
	m.wg.Wait()

	// Cancel context
	m.cancel()

	// Close result queue
	close(m.resultQueue)

	// Collect all results
	var results []Result
	for result := range m.resultQueue {
		results = append(results, result)
	}

	log.Info("ワーカーマネージャー停止完了",
		zap.Int("total_results", len(results)),
	)

	return results
}

// GetResults returns a channel to receive results
func (m *WorkerManager) GetResults() <-chan Result {
	return m.resultQueue
}

// GetStatus returns the status of all workers
func (m *WorkerManager) GetStatus() []WorkerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]WorkerStatus, 0, len(m.workers))
	for _, worker := range m.workers {
		statuses = append(statuses, worker.GetStatus())
	}

	return statuses
}

// GetQueueSize returns the current size of the task queue
func (m *WorkerManager) GetQueueSize() int {
	return len(m.taskQueue)
}

// IsRunning returns whether the manager is running
func (m *WorkerManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// AdjustWorkerCount dynamically adjusts the number of workers
func (m *WorkerManager) AdjustWorkerCount(delta int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return errors.New(errors.ErrInternal, "ワーカーマネージャーが実行されていません")
	}

	currentCount := len(m.workers)
	newCount := currentCount + delta

	if newCount < 1 {
		return errors.New(errors.ErrInvalidInput, "ワーカー数は1以上である必要があります")
	}

	if delta > 0 {
		// Add workers
		for i := 0; i < delta; i++ {
			workerID := currentCount + i
			worker := NewWorker(workerID, m.taskQueue, m.resultQueue, m.config)
			m.workers = append(m.workers, worker)
			m.wg.Add(1)
			go worker.Start(m.ctx, &m.wg)
		}
		log.Info("ワーカー追加",
			zap.Int("added", delta),
			zap.Int("total", newCount),
		)
	} else {
		// Note: Removing workers is more complex and would require
		// a mechanism to signal specific workers to stop
		log.Warn("ワーカー削減は現在サポートされていません",
			zap.Int("requested_delta", delta),
		)
	}

	return nil
}