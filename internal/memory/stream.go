package memory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// StreamProcessor processes data in streaming fashion for memory efficiency
type StreamProcessor struct {
	poolManager *PoolManager
	bufferSize  int
	maxLineSize int
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(bufferSize int) *StreamProcessor {
	return &StreamProcessor{
		poolManager: GetGlobalPoolManager(),
		bufferSize:  bufferSize,
		maxLineSize: 64 * 1024, // 64KB max line size
	}
}

// StreamOptions holds options for streaming operations
type StreamOptions struct {
	BufferSize     int
	MaxLineSize    int
	ChunkProcessor func([]byte) error
	LineProcessor  func(string) error
}

// DefaultStreamOptions returns default streaming options
func DefaultStreamOptions() StreamOptions {
	return StreamOptions{
		BufferSize:  4096,
		MaxLineSize: 64 * 1024,
	}
}

// ProcessReader processes data from a reader in streaming fashion
func (sp *StreamProcessor) ProcessReader(ctx context.Context, reader io.Reader, options StreamOptions) error {
	if options.BufferSize == 0 {
		options.BufferSize = sp.bufferSize
	}
	if options.MaxLineSize == 0 {
		options.MaxLineSize = sp.maxLineSize
	}

	// Get buffer from pool
	var buffer []byte
	if sp.poolManager != nil && sp.poolManager.GetBytePool() != nil {
		buffer = sp.poolManager.GetBytePool().Get(options.BufferSize)
		defer sp.poolManager.GetBytePool().Put(buffer)
	} else {
		buffer = make([]byte, options.BufferSize)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(buffer, options.MaxLineSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if options.LineProcessor != nil {
			if err := options.LineProcessor(line); err != nil {
				return fmt.Errorf("line processing error: %w", err)
			}
		}
	}

	return scanner.Err()
}

// ProcessReaderInChunks processes data from a reader in chunks
func (sp *StreamProcessor) ProcessReaderInChunks(ctx context.Context, reader io.Reader, options StreamOptions) error {
	if options.BufferSize == 0 {
		options.BufferSize = sp.bufferSize
	}

	// Get buffer from pool
	var buffer []byte
	if sp.poolManager != nil && sp.poolManager.GetBytePool() != nil {
		buffer = sp.poolManager.GetBytePool().Get(options.BufferSize)
		defer sp.poolManager.GetBytePool().Put(buffer)
	} else {
		buffer = make([]byte, options.BufferSize)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := reader.Read(buffer)
		if n > 0 && options.ChunkProcessor != nil {
			if err := options.ChunkProcessor(buffer[:n]); err != nil {
				return fmt.Errorf("chunk processing error: %w", err)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}

	return nil
}

// EnvFileStreamer handles streaming processing of .env files
type EnvFileStreamer struct {
	processor   *StreamProcessor
	poolManager *PoolManager
}

// NewEnvFileStreamer creates a new env file streamer
func NewEnvFileStreamer() *EnvFileStreamer {
	return &EnvFileStreamer{
		processor:   NewStreamProcessor(4096),
		poolManager: GetGlobalPoolManager(),
	}
}

// EnvVariable represents a single environment variable during streaming
type EnvVariable struct {
	Key     string
	Value   string
	Comment string
	Line    int
}

// StreamParseResult holds the result of streaming parsing
type StreamParseResult struct {
	Variables chan EnvVariable
	Errors    chan error
	Done      chan bool
}

// StreamParse parses an .env file in streaming fashion
func (efs *EnvFileStreamer) StreamParse(ctx context.Context, reader io.Reader) *StreamParseResult {
	result := &StreamParseResult{
		Variables: make(chan EnvVariable, 100),
		Errors:    make(chan error, 10),
		Done:      make(chan bool, 1),
	}

	go func() {
		defer close(result.Variables)
		defer close(result.Errors)
		defer func() { result.Done <- true }()

		lineNum := 0
		options := StreamOptions{
			BufferSize:  8192,
			MaxLineSize: 64 * 1024,
			LineProcessor: func(line string) error {
				lineNum++
				
				// Skip empty lines
				if strings.TrimSpace(line) == "" {
					return nil
				}

				// Skip comments
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					return nil
				}

				// Parse variable line
				if strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						
						var comment string
						if idx := strings.Index(value, " #"); idx != -1 {
							comment = strings.TrimSpace(value[idx+2:])
							value = strings.TrimSpace(value[:idx])
						}

						// Remove quotes
						value = efs.trimQuotes(value)

						select {
						case result.Variables <- EnvVariable{
							Key:     key,
							Value:   value,
							Comment: comment,
							Line:    lineNum,
						}:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}

				return nil
			},
		}

		if err := efs.processor.ProcessReader(ctx, reader, options); err != nil {
			select {
			case result.Errors <- err:
			case <-ctx.Done():
			}
		}
	}()

	return result
}

// trimQuotes removes surrounding quotes from a value
func (efs *EnvFileStreamer) trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// BatchProcessor processes data in batches for memory efficiency
type BatchProcessor struct {
	poolManager *PoolManager
	batchSize   int
	workers     int
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(batchSize, workers int) *BatchProcessor {
	return &BatchProcessor{
		poolManager: GetGlobalPoolManager(),
		batchSize:   batchSize,
		workers:     workers,
	}
}

// BatchJob represents a job to be processed in a batch
type BatchJob interface {
	Process() error
}

// ProcessBatch processes jobs in batches with multiple workers
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, jobs []BatchJob) error {
	if len(jobs) == 0 {
		return nil
	}

	jobChan := make(chan BatchJob, bp.batchSize)
	errorChan := make(chan error, bp.workers)
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < bp.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				if err := job.Process(); err != nil {
					select {
					case errorChan <- err:
					case <-ctx.Done():
					}
					return
				}
			}
		}()
	}

	// Send jobs to workers
	go func() {
		defer close(jobChan)
		for _, job := range jobs {
			select {
			case jobChan <- job:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	return ctx.Err()
}

// MemoryAwareWriter is a writer that monitors memory usage
type MemoryAwareWriter struct {
	writer      io.Writer
	poolManager *PoolManager
	threshold   int64
	written     int64
	bufferSize  int
}

// NewMemoryAwareWriter creates a new memory-aware writer
func NewMemoryAwareWriter(writer io.Writer, threshold int64, bufferSize int) *MemoryAwareWriter {
	return &MemoryAwareWriter{
		writer:      writer,
		poolManager: GetGlobalPoolManager(),
		threshold:   threshold,
		bufferSize:  bufferSize,
	}
}

// Write writes data while monitoring memory usage
func (maw *MemoryAwareWriter) Write(p []byte) (n int, err error) {
	// Check memory usage before writing
	if maw.poolManager != nil {
		stats := maw.poolManager.GetMemoryStats()
		if stats.HeapAlloc > uint64(maw.threshold) {
			// Force garbage collection if memory usage is high
			maw.poolManager.ForceGC()
		}
	}

	n, err = maw.writer.Write(p)
	maw.written += int64(n)
	return n, err
}

// WrittenBytes returns the number of bytes written
func (maw *MemoryAwareWriter) WrittenBytes() int64 {
	return maw.written
}

// LargeFileProcessor handles processing of large files efficiently
type LargeFileProcessor struct {
	poolManager *PoolManager
	chunkSize   int64
	maxMemory   int64
}

// NewLargeFileProcessor creates a new large file processor
func NewLargeFileProcessor(chunkSize, maxMemory int64) *LargeFileProcessor {
	return &LargeFileProcessor{
		poolManager: GetGlobalPoolManager(),
		chunkSize:   chunkSize,
		maxMemory:   maxMemory,
	}
}

// ProcessLargeFile processes a large file in chunks
func (lfp *LargeFileProcessor) ProcessLargeFile(ctx context.Context, reader io.Reader, processor func([]byte) error) error {
	// Get buffer from pool
	var buffer []byte
	if lfp.poolManager != nil && lfp.poolManager.GetBytePool() != nil {
		buffer = lfp.poolManager.GetBytePool().Get(int(lfp.chunkSize))
		defer lfp.poolManager.GetBytePool().Put(buffer)
	} else {
		buffer = make([]byte, lfp.chunkSize)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check memory usage
		if lfp.poolManager != nil {
			stats := lfp.poolManager.GetMemoryStats()
			if stats.HeapAlloc > uint64(lfp.maxMemory) {
				lfp.poolManager.ForceGC()
			}
		}

		n, err := reader.Read(buffer)
		if n > 0 {
			if err := processor(buffer[:n]); err != nil {
				return fmt.Errorf("processing error: %w", err)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}

	return nil
}

// StringBuilderPool manages a pool of string builders
type StringBuilderPool struct {
	pool    sync.Pool
	maxSize int64
}

// NewStringBuilderPool creates a new string builder pool
func NewStringBuilderPool(maxSize int64) *StringBuilderPool {
	return &StringBuilderPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &strings.Builder{}
			},
		},
		maxSize: maxSize,
	}
}

// Get returns a string builder from the pool
func (sbp *StringBuilderPool) Get() *strings.Builder {
	sb := sbp.pool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// Put returns a string builder to the pool
func (sbp *StringBuilderPool) Put(sb *strings.Builder) {
	if int64(sb.Cap()) <= sbp.maxSize {
		sbp.pool.Put(sb)
	}
}

// Global string builder pool
var globalStringBuilderPool *StringBuilderPool
var globalStringBuilderOnce sync.Once

// GetGlobalStringBuilderPool returns the global string builder pool
func GetGlobalStringBuilderPool() *StringBuilderPool {
	globalStringBuilderOnce.Do(func() {
		globalStringBuilderPool = NewStringBuilderPool(64 * 1024) // 64KB max
	})
	return globalStringBuilderPool
}