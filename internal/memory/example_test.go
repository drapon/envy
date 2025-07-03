package memory_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/drapon/envy/internal/memory"
)

// ExampleStringPool demonstrates how to use the string pool
func ExampleStringPool() {
	pool := memory.NewStringPool(1024)
	
	// Get a string slice from the pool
	slice := pool.Get()
	
	// Use the slice
	slice = append(slice, "example", "string", "data")
	
	// Process the slice
	fmt.Printf("Processed %d strings\n", len(slice))
	
	// Return the slice to the pool
	pool.Put(slice)
	
	// Get statistics
	stats := pool.Stats()
	fmt.Printf("Pool stats: Gets=%d, Puts=%d, Hits=%d\n", stats.Gets, stats.Puts, stats.Hits)
	
	// Output:
	// Processed 3 strings
	// Pool stats: Gets=1, Puts=1, Hits=0
}

// ExampleBytePool demonstrates how to use the byte pool
func ExampleBytePool() {
	pool := memory.NewBytePool(64 * 1024)
	
	// Get a byte slice from the pool
	buffer := pool.Get(1024)
	
	// Use the buffer
	copy(buffer, []byte("example data"))
	fmt.Printf("Buffer size: %d bytes\n", len(buffer))
	
	// Return the buffer to the pool
	pool.Put(buffer)
	
	// Output:
	// Buffer size: 1024 bytes
}

// ExampleStreamProcessor demonstrates streaming processing
func ExampleStreamProcessor() {
	processor := memory.NewStreamProcessor(4096)
	
	// Sample data
	data := strings.NewReader("KEY1=value1\nKEY2=value2\nKEY3=value3\n")
	
	// Process with line callback
	ctx := context.Background()
	options := memory.StreamOptions{
		BufferSize: 1024,
		LineProcessor: func(line string) error {
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				fmt.Printf("Found variable: %s=%s\n", parts[0], parts[1])
			}
			return nil
		},
	}
	
	err := processor.ProcessReader(ctx, data, options)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	
	// Output:
	// Found variable: KEY1=value1
	// Found variable: KEY2=value2
	// Found variable: KEY3=value3
}

// ExamplePoolManager demonstrates the pool manager
func ExamplePoolManager() {
	config := memory.PoolConfig{
		Enabled:          true,
		EnableMonitoring: true,
		StringPoolSize:   1024,
		BytePoolSize:     64 * 1024,
		MapPoolSize:      100,
		GCInterval:       30 * time.Second,
		MemoryThreshold:  100 * 1024 * 1024,
	}
	
	manager := memory.NewPoolManager(config)
	defer manager.Close()
	
	// Use the pools
	stringPool := manager.GetStringPool()
	bytePool := manager.GetBytePool()
	mapPool := manager.GetMapPool()
	
	if stringPool != nil && bytePool != nil && mapPool != nil {
		fmt.Println("All pools are available")
	}
	
	// Get memory statistics
	memStats := manager.GetMemoryStats()
	fmt.Printf("Memory allocated: %d bytes\n", memStats.Alloc)
	
	// Get pool statistics
	poolStats := manager.GetAllStats()
	fmt.Printf("Number of pools: %d\n", len(poolStats))
	
	// Output:
	// All pools are available
	// Memory allocated: 0 bytes
	// Number of pools: 3
}

// BenchmarkStringPool benchmarks string pool performance
func BenchmarkStringPool(b *testing.B) {
	pool := memory.NewStringPool(1024)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slice := pool.Get()
		slice = append(slice, "test", "data", "benchmark")
		pool.Put(slice)
	}
}

// BenchmarkBytePool benchmarks byte pool performance
func BenchmarkBytePool(b *testing.B) {
	pool := memory.NewBytePool(64 * 1024)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer := pool.Get(1024)
		copy(buffer, []byte("benchmark data"))
		pool.Put(buffer)
	}
}

// BenchmarkMapPool benchmarks map pool performance
func BenchmarkMapPool(b *testing.B) {
	pool := memory.NewMapPool(100)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := pool.Get()
		m["key1"] = "value1"
		m["key2"] = "value2"
		pool.Put(m)
	}
}

// BenchmarkStreamProcessor benchmarks streaming processor performance
func BenchmarkStreamProcessor(b *testing.B) {
	processor := memory.NewStreamProcessor(4096)
	
	// Large test data
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString(fmt.Sprintf("KEY%d=value%d\n", i, i))
	}
	
	data := builder.String()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(data)
		ctx := context.Background()
		
		options := memory.StreamOptions{
			BufferSize: 8192,
			LineProcessor: func(line string) error {
				// Simulate processing
				_ = strings.Contains(line, "=")
				return nil
			},
		}
		
		err := processor.ProcessReader(ctx, reader, options)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

// TestMemoryOptimization tests memory optimization features
func TestMemoryOptimization(t *testing.T) {
	// Test pool manager creation
	config := memory.DefaultPoolConfig()
	manager := memory.NewPoolManager(config)
	defer manager.Close()
	
	// Test string pool
	stringPool := manager.GetStringPool()
	if stringPool == nil {
		t.Fatal("String pool should not be nil")
	}
	
	slice := stringPool.Get()
	slice = append(slice, "test")
	stringPool.Put(slice)
	
	stats := stringPool.Stats()
	if stats.Gets != 1 || stats.Puts != 1 {
		t.Errorf("Expected Gets=1, Puts=1, got Gets=%d, Puts=%d", stats.Gets, stats.Puts)
	}
	
	// Test byte pool
	bytePool := manager.GetBytePool()
	if bytePool == nil {
		t.Fatal("Byte pool should not be nil")
	}
	
	buffer := bytePool.Get(1024)
	if len(buffer) != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", len(buffer))
	}
	bytePool.Put(buffer)
	
	// Test map pool
	mapPool := manager.GetMapPool()
	if mapPool == nil {
		t.Fatal("Map pool should not be nil")
	}
	
	m := mapPool.Get()
	m["test"] = "value"
	mapPool.Put(m)
	
	// Test memory statistics
	memStats := manager.GetMemoryStats()
	if memStats.Alloc < 0 {
		t.Errorf("Memory allocation should be non-negative, got %d", memStats.Alloc)
	}
}

// TestStreamingProcessor tests streaming processor functionality
func TestStreamingProcessor(t *testing.T) {
	processor := memory.NewStreamProcessor(4096)
	
	// Test data
	data := strings.NewReader("KEY1=value1\nKEY2=value2\n# Comment\nKEY3=value3\n")
	
	var processedLines []string
	ctx := context.Background()
	
	options := memory.StreamOptions{
		BufferSize: 1024,
		LineProcessor: func(line string) error {
			processedLines = append(processedLines, line)
			return nil
		},
	}
	
	err := processor.ProcessReader(ctx, data, options)
	if err != nil {
		t.Fatalf("Error processing stream: %v", err)
	}
	
	expectedLines := 4 // All lines including comment
	if len(processedLines) != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, len(processedLines))
	}
}

// TestBatchProcessor tests batch processing functionality
func TestBatchProcessor(t *testing.T) {
	processor := memory.NewBatchProcessor(10, 2)
	
	// Create test jobs
	jobs := make([]memory.BatchJob, 20)
	for i := 0; i < 20; i++ {
		jobs[i] = &testJob{id: i}
	}
	
	ctx := context.Background()
	err := processor.ProcessBatch(ctx, jobs)
	if err != nil {
		t.Fatalf("Error processing batch: %v", err)
	}
	
	// Verify all jobs were processed
	for i, job := range jobs {
		testJob := job.(*testJob)
		if !testJob.processed {
			t.Errorf("Job %d was not processed", i)
		}
	}
}

// testJob implements BatchJob for testing
type testJob struct {
	id        int
	processed bool
}

func (j *testJob) Process() error {
	j.processed = true
	return nil
}