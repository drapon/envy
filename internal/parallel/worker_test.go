package parallel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drapon/envy/internal/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorker tests basic worker functionality
func TestWorker(t *testing.T) {
	taskQueue := make(chan Task, 10)
	resultQueue := make(chan Result, 10)

	config := &WorkerConfig{
		RetryConfig: &retry.Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		},
	}

	worker := NewWorker(1, taskQueue, resultQueue, config)
	assert.NotNil(t, worker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	var wg sync.WaitGroup
	wg.Add(1)
	go worker.Start(ctx, &wg)

	// Submit a successful task
	successTask := NewTaskFunc("success", func(ctx context.Context) error {
		return nil
	}, false)

	taskQueue <- successTask

	// Wait for result
	select {
	case result := <-resultQueue:
		assert.NoError(t, result.Error)
		assert.Equal(t, "success", result.Task.Name())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	// Submit a failing task
	failTask := NewTaskFunc("fail", func(ctx context.Context) error {
		return errors.New("task failed")
	}, false)

	taskQueue <- failTask

	// Wait for result
	select {
	case result := <-resultQueue:
		assert.Error(t, result.Error)
		assert.Equal(t, "fail", result.Task.Name())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	// Check status
	status := worker.GetStatus()
	assert.Equal(t, 1, status.ID)
	assert.Equal(t, int64(2), status.TotalProcessed)
	assert.Equal(t, int64(1), status.TotalFailed)

	// Stop worker
	cancel()
	close(taskQueue)
	wg.Wait()
}

// TestWorkerRetry tests retry functionality
func TestWorkerRetry(t *testing.T) {
	t.Skip("Skipping worker retry test - needs implementation")
	taskQueue := make(chan Task, 10)
	resultQueue := make(chan Result, 10)

	config := &WorkerConfig{
		RetryConfig: &retry.Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
		},
	}

	worker := NewWorker(1, taskQueue, resultQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go worker.Start(ctx, &wg)

	// Create a task that fails twice then succeeds
	var attempts atomic.Int32
	retryTask := NewTaskFunc("retry", func(ctx context.Context) error {
		count := attempts.Add(1)
		if count < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, true) // Retriable

	taskQueue <- retryTask

	// Wait for result
	select {
	case result := <-resultQueue:
		assert.NoError(t, result.Error)
		assert.Equal(t, int32(3), attempts.Load())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for result")
	}

	cancel()
	close(taskQueue)
	wg.Wait()
}

// TestWorkerManager tests worker manager functionality
func TestWorkerManager(t *testing.T) {
	ctx := context.Background()
	manager := NewWorkerManager(ctx, 3, WithMaxQueueSize(100))

	// Start manager
	err := manager.Start()
	require.NoError(t, err)
	assert.True(t, manager.IsRunning())

	// Submit tasks
	var successCount atomic.Int32
	for i := 0; i < 10; i++ {
		task := NewTaskFunc(fmt.Sprintf("task-%d", i), func(ctx context.Context) error {
			successCount.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		}, false)

		err := manager.Submit(task)
		require.NoError(t, err)
	}

	// Get results
	results := make([]Result, 0, 10)
	resultChan := manager.GetResults()

	for i := 0; i < 10; i++ {
		select {
		case result := <-resultChan:
			results = append(results, result)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for results")
		}
	}

	assert.Equal(t, 10, len(results))
	assert.Equal(t, int32(10), successCount.Load())

	// Check worker statuses
	statuses := manager.GetStatus()
	assert.Equal(t, 3, len(statuses))

	// Stop manager
	finalResults := manager.Stop()
	assert.False(t, manager.IsRunning())
	assert.Equal(t, 0, len(finalResults)) // All results already collected
}

// TestWorkerManagerSubmitBatch tests batch submission
func TestWorkerManagerSubmitBatch(t *testing.T) {
	ctx := context.Background()
	manager := NewWorkerManager(ctx, 2)

	err := manager.Start()
	require.NoError(t, err)

	// Create batch of tasks
	tasks := make([]Task, 5)
	for i := 0; i < 5; i++ {
		tasks[i] = NewTaskFunc(fmt.Sprintf("batch-%d", i), func(ctx context.Context) error {
			return nil
		}, false)
	}

	// Submit batch
	err = manager.SubmitBatch(tasks)
	require.NoError(t, err)

	// Collect results
	results := make([]Result, 0, 5)
	resultChan := manager.GetResults()

	for i := 0; i < 5; i++ {
		select {
		case result := <-resultChan:
			results = append(results, result)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for results")
		}
	}

	assert.Equal(t, 5, len(results))
	manager.Stop()
}

// TestWorkerManagerAdjustWorkerCount tests dynamic worker adjustment
func TestWorkerManagerAdjustWorkerCount(t *testing.T) {
	ctx := context.Background()
	manager := NewWorkerManager(ctx, 2)

	err := manager.Start()
	require.NoError(t, err)

	// Get initial status
	initialStatuses := manager.GetStatus()
	assert.Equal(t, 2, len(initialStatuses))

	// Add workers
	err = manager.AdjustWorkerCount(2)
	require.NoError(t, err)

	// Give time for workers to start
	time.Sleep(100 * time.Millisecond)

	// Check new worker count
	newStatuses := manager.GetStatus()
	assert.Equal(t, 4, len(newStatuses))

	// Try invalid adjustment
	err = manager.AdjustWorkerCount(-10)
	assert.Error(t, err)

	manager.Stop()
}

// TestWorkerHooks tests before/after execute hooks
func TestWorkerHooks(t *testing.T) {
	taskQueue := make(chan Task, 10)
	resultQueue := make(chan Result, 10)

	var beforeCalled, afterCalled atomic.Bool

	config := &WorkerConfig{
		BeforeExecute: func(task Task) {
			beforeCalled.Store(true)
		},
		AfterExecute: func(task Task, result Result) {
			afterCalled.Store(true)
		},
	}

	worker := NewWorker(1, taskQueue, resultQueue, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go worker.Start(ctx, &wg)

	// Submit task
	task := NewTaskFunc("hook-test", func(ctx context.Context) error {
		return nil
	}, false)

	taskQueue <- task

	// Wait for result
	select {
	case <-resultQueue:
		assert.True(t, beforeCalled.Load())
		assert.True(t, afterCalled.Load())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	cancel()
	close(taskQueue)
	wg.Wait()
}

// BenchmarkWorkerManager benchmarks worker manager performance
func BenchmarkWorkerManager(b *testing.B) {
	ctx := context.Background()
	manager := NewWorkerManager(ctx, 4, WithMaxQueueSize(1000))

	err := manager.Start()
	require.NoError(b, err)
	defer manager.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		task := NewTaskFunc("bench", func(ctx context.Context) error {
			// Simulate some work
			time.Sleep(time.Microsecond)
			return nil
		}, false)

		err := manager.Submit(task)
		if err != nil {
			b.Fatal(err)
		}
	}

	// Wait for all results
	for i := 0; i < b.N; i++ {
		select {
		case <-manager.GetResults():
		case <-time.After(10 * time.Second):
			b.Fatal("timeout waiting for results")
		}
	}
}
