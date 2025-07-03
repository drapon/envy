package parallel

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProgressTracker tests progress tracking functionality
func TestProgressTracker(t *testing.T) {
	tracker := NewProgressTracker(10, "テスト処理", true)

	// Track some successful operations
	for i := 0; i < 5; i++ {
		tracker.Increment()
	}

	// Track some failed operations
	for i := 0; i < 3; i++ {
		tracker.IncrementWithError(errors.New("test error"))
	}

	// Check stats
	completed, failed, duration := tracker.GetStats()
	assert.Equal(t, int64(8), completed)
	assert.Equal(t, int64(3), failed)
	assert.Greater(t, duration, time.Duration(0))

	// Finish tracking
	tracker.Finish()
}

// TestProgressTask tests progress-tracked task execution
func TestProgressTask(t *testing.T) {
	ctx := context.Background()
	tracker := NewProgressTracker(2, "タスク実行", false)

	// Create successful task
	successTask := NewTaskFunc("success", func(ctx context.Context) error {
		return nil
	}, false)
	progressTask1 := NewProgressTask(successTask, tracker)

	// Execute successful task
	err := progressTask1.Execute(ctx)
	assert.NoError(t, err)

	// Create failing task
	failTask := NewTaskFunc("fail", func(ctx context.Context) error {
		return errors.New("task failed")
	}, false)
	progressTask2 := NewProgressTask(failTask, tracker)

	// Execute failing task
	err = progressTask2.Execute(ctx)
	assert.Error(t, err)

	// Check tracker stats
	completed, failed, _ := tracker.GetStats()
	assert.Equal(t, int64(2), completed)
	assert.Equal(t, int64(1), failed)
}

// TestProgressPool tests progress-tracked worker pool
func TestProgressPool(t *testing.T) {
	ctx := context.Background()
	pool := NewProgressPool(ctx, 10, "並列処理テスト",
		WithMaxWorkers(3),
	)

	pool.Start()

	// Submit tasks
	for i := 0; i < 10; i++ {
		task := NewTaskFunc(fmt.Sprintf("task-%d", i), func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			if i%3 == 0 {
				return errors.New("simulated error")
			}
			return nil
		}, false)

		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Wait for completion
	results := pool.Wait()
	assert.Equal(t, 10, len(results))

	// Progress tracker should have finished and shown stats
}

// TestBatchProgressProcessor tests batch processing with progress
func TestBatchProgressProcessor(t *testing.T) {
	ctx := context.Background()
	processor := NewBatchProgressProcessor(ctx, 2, true,
		WithBatchSize(5),
	)

	// Create items
	items := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		items[i] = i
	}

	// Process with progress
	results, err := processor.ProcessWithProgress(
		ctx,
		items,
		"アイテム処理中",
		func(ctx context.Context, item interface{}) error {
			time.Sleep(10 * time.Millisecond)
			// Fail every 5th item
			if item.(int)%5 == 0 {
				return fmt.Errorf("failed for item %v", item)
			}
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 20, len(results))

	// Count errors
	var errorCount int
	for _, result := range results {
		if result.Error != nil {
			errorCount++
		}
	}
	assert.Equal(t, 4, errorCount) // 0, 5, 10, 15
}

// TestMonitoredPool tests monitored pool functionality
func TestMonitoredPool(t *testing.T) {
	ctx := context.Background()
	pool := NewMonitoredPool(ctx, 100*time.Millisecond,
		WithMaxWorkers(2),
	)

	pool.Start()

	// Submit tasks
	for i := 0; i < 10; i++ {
		task := NewTaskFunc(fmt.Sprintf("monitored-%d", i), func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}, false)

		err := pool.Submit(task)
		require.NoError(t, err)
	}

	// Let monitoring run for a bit
	time.Sleep(200 * time.Millisecond)

	// Stop pool
	pool.Stop()

	// Wait for tasks
	results := pool.Wait()
	assert.Equal(t, 10, len(results))
}

// TestProgressReporter tests progress reporting
func TestProgressReporter(t *testing.T) {
	reporter := NewProgressReporter()

	// Start operations
	reporter.StartOperation("operation1", 10)
	reporter.StartOperation("operation2", 5)

	// Update operations
	reporter.UpdateOperation("operation1", 5, 1)
	reporter.UpdateOperation("operation2", 3, 0)

	// Get report
	report := reporter.GetReport()
	assert.Contains(t, report, "進捗レポート")
	assert.Contains(t, report, "operation1")
	assert.Contains(t, report, "operation2")

	// Complete an operation
	reporter.UpdateOperation("operation2", 5, 0)

	// Check report again
	report = reporter.GetReport()
	assert.Contains(t, report, "1/2 操作完了")
}

// TestProcessWithProgress tests the standalone progress processing function
func TestProcessWithProgress(t *testing.T) {
	ctx := context.Background()

	// Create tasks
	tasks := make([]Task, 5)
	for i := 0; i < 5; i++ {
		tasks[i] = NewTaskFunc(fmt.Sprintf("progress-%d", i), func(ctx context.Context) error {
			time.Sleep(20 * time.Millisecond)
			return nil
		}, false)
	}

	// Process with progress
	results, err := ProcessWithProgress(
		ctx,
		tasks,
		func(ctx context.Context, task Task) error {
			return task.Execute(ctx)
		},
		"タスク処理中",
	)

	require.NoError(t, err)
	assert.Equal(t, 5, len(results))

	// All should succeed
	for _, result := range results {
		assert.NoError(t, result.Error)
	}
}

// BenchmarkProgressTracker benchmarks progress tracking overhead
func BenchmarkProgressTracker(b *testing.B) {
	tracker := NewProgressTracker(b.N, "ベンチマーク", false)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if i%10 == 0 {
			tracker.IncrementWithError(errors.New("error"))
		} else {
			tracker.Increment()
		}
	}

	tracker.Finish()
}

// BenchmarkProgressPool benchmarks progress pool performance
func BenchmarkProgressPool(b *testing.B) {
	ctx := context.Background()
	pool := NewProgressPool(ctx, b.N, "ベンチマーク処理",
		WithMaxWorkers(4),
		WithBufferSize(1000),
	)

	pool.Start()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		task := NewTaskFunc("bench", func(ctx context.Context) error {
			return nil
		}, false)

		err := pool.Submit(task)
		if err != nil {
			b.Fatal(err)
		}
	}

	pool.Wait()
}
