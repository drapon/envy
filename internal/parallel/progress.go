package parallel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/drapon/envy/internal/log"
	"github.com/schollz/progressbar/v3"
	"go.uber.org/zap"
)

// ProgressTracker tracks progress of parallel operations
type ProgressTracker struct {
	total       int64
	completed   atomic.Int64
	failed      atomic.Int64
	bar         *progressbar.ProgressBar
	mu          sync.Mutex
	startTime   time.Time
	showDetails bool
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(total int, description string, showDetails bool) *ProgressTracker {
	options := []progressbar.Option{
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("items"),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]█[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	}

	if showDetails {
		options = append(options, 
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionSetPredictTime(true),
		)
	}

	bar := progressbar.NewOptions64(int64(total), options...)

	return &ProgressTracker{
		total:       int64(total),
		bar:         bar,
		startTime:   time.Now(),
		showDetails: showDetails,
	}
}

// Increment increments the progress
func (p *ProgressTracker) Increment() {
	p.completed.Add(1)
	p.bar.Add(1)
}

// IncrementWithError increments progress and marks as failed
func (p *ProgressTracker) IncrementWithError(err error) {
	p.failed.Add(1)
	p.completed.Add(1)
	p.bar.Add(1)
	
	if p.showDetails {
		p.mu.Lock()
		defer p.mu.Unlock()
		
		// Update description to show errors
		failed := p.failed.Load()
		completed := p.completed.Load()
		p.bar.Describe(fmt.Sprintf("処理中... (完了: %d/%d, エラー: %d)",
			completed, p.total, failed))
	}
}

// Finish finishes the progress tracking
func (p *ProgressTracker) Finish() {
	p.bar.Finish()
	
	duration := time.Since(p.startTime)
	completed := p.completed.Load()
	failed := p.failed.Load()
	succeeded := completed - failed
	
	// Show summary
	fmt.Printf("\n処理完了: 成功 %d, 失敗 %d, 合計 %d (%.2f秒)\n",
		succeeded, failed, completed, duration.Seconds())
	
	// Calculate throughput
	if duration > 0 {
		throughput := float64(completed) / duration.Seconds()
		fmt.Printf("スループット: %.2f items/秒\n", throughput)
	}
}

// GetStats returns current statistics
func (p *ProgressTracker) GetStats() (completed, failed int64, duration time.Duration) {
	return p.completed.Load(), p.failed.Load(), time.Since(p.startTime)
}

// ProgressTask wraps a task with progress tracking
type ProgressTask struct {
	Task
	tracker *ProgressTracker
}

// NewProgressTask creates a new progress-tracked task
func NewProgressTask(task Task, tracker *ProgressTracker) *ProgressTask {
	return &ProgressTask{
		Task:    task,
		tracker: tracker,
	}
}

// Execute executes the task and updates progress
func (t *ProgressTask) Execute(ctx context.Context) error {
	err := t.Task.Execute(ctx)
	
	if err != nil {
		t.tracker.IncrementWithError(err)
	} else {
		t.tracker.Increment()
	}
	
	return err
}

// ProgressPool is a worker pool with progress tracking
type ProgressPool struct {
	*WorkerPool
	tracker *ProgressTracker
}

// NewProgressPool creates a new progress-tracked worker pool
func NewProgressPool(ctx context.Context, total int, description string, opts ...PoolOption) *ProgressPool {
	pool := NewWorkerPool(ctx, opts...)
	tracker := NewProgressTracker(total, description, true)
	
	return &ProgressPool{
		WorkerPool: pool,
		tracker:    tracker,
	}
}

// Submit submits a task with progress tracking
func (p *ProgressPool) Submit(task Task) error {
	progressTask := NewProgressTask(task, p.tracker)
	return p.WorkerPool.Submit(progressTask)
}

// Wait waits for completion and shows final stats
func (p *ProgressPool) Wait() []Result {
	results := p.WorkerPool.Wait()
	p.tracker.Finish()
	return results
}

// BatchProgressProcessor combines batch processing with progress tracking
type BatchProgressProcessor struct {
	*BatchProcessor
	showProgress bool
}

// NewBatchProgressProcessor creates a batch processor with progress tracking
func NewBatchProgressProcessor(
	ctx context.Context,
	maxWorkers int,
	showProgress bool,
	opts ...BatchOption,
) *BatchProgressProcessor {
	processor := NewBatchProcessor(ctx, maxWorkers, opts...)
	
	return &BatchProgressProcessor{
		BatchProcessor: processor,
		showProgress:   showProgress,
	}
}

// ProcessWithProgress processes items with progress tracking
func (b *BatchProgressProcessor) ProcessWithProgress(
	ctx context.Context,
	items []interface{},
	description string,
	fn ProcessFunc,
) ([]Result, error) {
	if !b.showProgress {
		return b.Process(ctx, items, fn)
	}

	// Create progress tracker
	tracker := NewProgressTracker(len(items), description, true)
	
	// Wrap process function with progress tracking
	progressFn := func(ctx context.Context, item interface{}) error {
		err := fn(ctx, item)
		
		if err != nil {
			tracker.IncrementWithError(err)
		} else {
			tracker.Increment()
		}
		
		return err
	}

	// Process items
	results, err := b.Process(ctx, items, progressFn)
	
	// Finish progress tracking
	tracker.Finish()
	
	return results, err
}

// MonitoredPool provides real-time monitoring of pool metrics
type MonitoredPool struct {
	*WorkerPool
	interval time.Duration
	stopChan chan struct{}
}

// NewMonitoredPool creates a pool with monitoring
func NewMonitoredPool(ctx context.Context, interval time.Duration, opts ...PoolOption) *MonitoredPool {
	pool := NewWorkerPool(ctx, opts...)
	
	return &MonitoredPool{
		WorkerPool: pool,
		interval:   interval,
		stopChan:   make(chan struct{}),
	}
}

// Start starts the pool with monitoring
func (m *MonitoredPool) Start() {
	m.WorkerPool.Start()
	go m.monitor()
}

// Stop stops the pool and monitoring
func (m *MonitoredPool) Stop() {
	close(m.stopChan)
	m.WorkerPool.Stop()
}

// monitor continuously monitors pool metrics
func (m *MonitoredPool) monitor() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			processed, failed, activeWorkers := m.GetMetrics()
			
			log.Debug("プール統計",
				zap.Int64("processed", processed),
				zap.Int64("failed", failed),
				zap.Int32("active_workers", activeWorkers),
				zap.Int("queue_size", len(m.tasks)),
			)
		}
	}
}

// ProgressReporter provides detailed progress reporting
type ProgressReporter struct {
	mu          sync.Mutex
	operations  map[string]*OperationProgress
	totalOps    int
	completedOps int
}

// OperationProgress tracks progress of a specific operation
type OperationProgress struct {
	Name      string
	Total     int
	Completed int
	Failed    int
	StartTime time.Time
	EndTime   time.Time
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{
		operations: make(map[string]*OperationProgress),
	}
}

// StartOperation starts tracking a new operation
func (r *ProgressReporter) StartOperation(name string, total int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.operations[name] = &OperationProgress{
		Name:      name,
		Total:     total,
		StartTime: time.Now(),
	}
	r.totalOps++
}

// UpdateOperation updates operation progress
func (r *ProgressReporter) UpdateOperation(name string, completed, failed int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if op, exists := r.operations[name]; exists {
		op.Completed = completed
		op.Failed = failed
		
		if completed >= op.Total {
			op.EndTime = time.Now()
			r.completedOps++
		}
	}
}

// GetReport generates a progress report
func (r *ProgressReporter) GetReport() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	report := fmt.Sprintf("進捗レポート: %d/%d 操作完了\n", r.completedOps, r.totalOps)
	
	for _, op := range r.operations {
		status := "進行中"
		if !op.EndTime.IsZero() {
			status = fmt.Sprintf("完了 (%.2f秒)", op.EndTime.Sub(op.StartTime).Seconds())
		}
		
		report += fmt.Sprintf("  %s: %d/%d (失敗: %d) - %s\n",
			op.Name, op.Completed, op.Total, op.Failed, status)
	}
	
	return report
}

// PrintReport prints the progress report
func (r *ProgressReporter) PrintReport() {
	fmt.Print(r.GetReport())
}

// ProcessWithProgress processes tasks with progress tracking
func ProcessWithProgress(ctx context.Context, tasks []Task, fn func(context.Context, Task) error, description string) ([]Result, error) {
	// Create progress tracker
	tracker := NewProgressTracker(len(tasks), description, true)
	
	// Create results slice
	results := make([]Result, len(tasks))
	
	// Process tasks
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t Task) {
			defer wg.Done()
			
			// Execute task
			err := fn(ctx, t)
			
			// Update progress
			if err != nil {
				tracker.IncrementWithError(err)
			} else {
				tracker.Increment()
			}
			
			// Store result
			results[index] = Result{
				Task:  t,
				Error: err,
			}
		}(i, task)
	}
	
	// Wait for all tasks to complete
	wg.Wait()
	
	// Finish progress tracking
	tracker.Finish()
	
	return results, nil
}