package env

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/drapon/envy/internal/memory"
)

// Variable represents an environment variable with metadata
type Variable struct {
	Key     string
	Value   string
	Comment string
	Line    int
}

// File represents a parsed .env file
type File struct {
	Variables map[string]*Variable
	Order     []string // Maintains original order
	Comments  map[int]string // Line number to comment mapping
}

// NewFile creates a new File instance
func NewFile() *File {
	return &File{
		Variables: make(map[string]*Variable),
		Order:     []string{},
		Comments:  make(map[int]string),
	}
}

// Parse parses an .env file from a reader
func Parse(r io.Reader) (*File, error) {
	return ParseWithContext(context.Background(), r)
}

// ParseWithContext parses an .env file from a reader with context
func ParseWithContext(ctx context.Context, r io.Reader) (*File, error) {
	file := NewFile()
	poolManager := memory.GetGlobalPoolManager()
	
	// Use memory pool for buffer if available
	var buffer []byte
	if poolManager != nil && poolManager.GetBytePool() != nil {
		buffer = poolManager.GetBytePool().Get(8192)
		defer poolManager.GetBytePool().Put(buffer)
	}
	
	scanner := bufio.NewScanner(r)
	if buffer != nil {
		scanner.Buffer(buffer, 64*1024) // 64KB max line size
	}
	
	lineNum := 0

	// Regex patterns - compiled once
	varPattern := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)
	commentPattern := regexp.MustCompile(`^\s*#(.*)$`)
	emptyPattern := regexp.MustCompile(`^\s*$`)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		lineNum++
		line := scanner.Text()

		// Handle empty lines
		if emptyPattern.MatchString(line) {
			continue
		}

		// Handle comments
		if matches := commentPattern.FindStringSubmatch(line); matches != nil {
			file.Comments[lineNum] = strings.TrimSpace(matches[1])
			continue
		}

		// Handle variable definitions
		if matches := varPattern.FindStringSubmatch(line); matches != nil {
			key := matches[1]
			value := matches[2]
			
			// Handle inline comments
			var comment string
			if idx := strings.Index(value, " #"); idx != -1 {
				comment = strings.TrimSpace(value[idx+2:])
				value = strings.TrimSpace(value[:idx])
			}

			// Remove quotes if present
			value = trimQuotes(value)

			variable := &Variable{
				Key:     key,
				Value:   value,
				Comment: comment,
				Line:    lineNum,
			}

			file.Variables[key] = variable
			file.Order = append(file.Order, key)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return file, nil
}

// ParseFile parses an .env file from disk
func ParseFile(filename string) (*File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	return Parse(f)
}

// Write writes the file content to a writer
func (f *File) Write(w io.Writer) error {
	return f.WriteWithContext(context.Background(), w)
}

// WriteWithContext writes the file content to a writer with context
func (f *File) WriteWithContext(ctx context.Context, w io.Writer) error {
	// Use memory-aware writer if threshold is set
	maw := memory.NewMemoryAwareWriter(w, 50*1024*1024, 8192) // 50MB threshold
	
	// Build a map of line numbers to content
	lines := make(map[int]string, len(f.Variables)+len(f.Comments))
	maxLine := 0

	// Add comments
	for lineNum, comment := range f.Comments {
		lines[lineNum] = fmt.Sprintf("# %s", comment)
		if lineNum > maxLine {
			maxLine = lineNum
		}
	}

	// Use string builder pool for efficient string concatenation
	sbPool := memory.GetGlobalStringBuilderPool()
	sb := sbPool.Get()
	defer sbPool.Put(sb)

	// Add variables in order
	for _, key := range f.Order {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if variable, ok := f.Variables[key]; ok {
			sb.Reset()
			sb.WriteString(variable.Key)
			sb.WriteString("=")
			sb.WriteString(formatValue(variable.Value))
			if variable.Comment != "" {
				sb.WriteString(" # ")
				sb.WriteString(variable.Comment)
			}
			lines[variable.Line] = sb.String()
			if variable.Line > maxLine {
				maxLine = variable.Line
			}
		}
	}

	// Write lines in order
	for i := 1; i <= maxLine; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if content, ok := lines[i]; ok {
			if _, err := fmt.Fprintln(maw, content); err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteFile writes the file content to disk
func (f *File) WriteFile(filename string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return f.Write(file)
}

// Get returns the value of a variable
func (f *File) Get(key string) (string, bool) {
	if variable, ok := f.Variables[key]; ok {
		return variable.Value, true
	}
	return "", false
}


// Set sets or updates a variable
func (f *File) Set(key, value string) {
	if variable, ok := f.Variables[key]; ok {
		variable.Value = value
	} else {
		f.Variables[key] = &Variable{
			Key:   key,
			Value: value,
			Line:  len(f.Variables) + len(f.Comments) + 1,
		}
		f.Order = append(f.Order, key)
	}
}

// Delete removes a variable
func (f *File) Delete(key string) {
	delete(f.Variables, key)
	// Remove from order
	for i, k := range f.Order {
		if k == key {
			f.Order = append(f.Order[:i], f.Order[i+1:]...)
			break
		}
	}
}

// ToMap converts variables to a simple map
func (f *File) ToMap() map[string]string {
	// Use memory pool for map if available
	poolManager := memory.GetGlobalPoolManager()
	var result map[string]string
	
	if poolManager != nil && poolManager.GetMapPool() != nil {
		result = poolManager.GetMapPool().Get()
		// Note: caller should return the map to pool when done
	} else {
		result = make(map[string]string, len(f.Variables))
	}
	
	for key, variable := range f.Variables {
		result[key] = variable.Value
	}
	return result
}

// ToMapWithPool converts variables to a simple map using memory pool
func (f *File) ToMapWithPool() (map[string]string, func()) {
	poolManager := memory.GetGlobalPoolManager()
	var result map[string]string
	var cleanup func()
	
	if poolManager != nil && poolManager.GetMapPool() != nil {
		result = poolManager.GetMapPool().Get()
		cleanup = func() { poolManager.GetMapPool().Put(result) }
	} else {
		result = make(map[string]string, len(f.Variables))
		cleanup = func() {} // No-op
	}
	
	for key, variable := range f.Variables {
		result[key] = variable.Value
	}
	return result, cleanup
}

// Keys returns all variable keys in order
func (f *File) Keys() []string {
	return append([]string{}, f.Order...)
}

// SortedKeys returns all variable keys sorted alphabetically
func (f *File) SortedKeys() []string {
	keys := f.Keys()
	sort.Strings(keys)
	return keys
}

// Merge merges another file into this one
func (f *File) Merge(other *File) {
	for _, key := range other.Order {
		if variable, ok := other.Variables[key]; ok {
			f.Set(key, variable.Value)
			if v, exists := f.Variables[key]; exists && variable.Comment != "" {
				v.Comment = variable.Comment
			}
		}
	}
}

// trimQuotes removes surrounding quotes from a value
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// formatValue adds quotes if necessary
func formatValue(value string) string {
	// Check if value needs quotes
	needsQuotes := false
	if strings.ContainsAny(value, " \t#\"'") || value == "" {
		needsQuotes = true
	}

	if needsQuotes {
		// Escape existing quotes
		value = strings.ReplaceAll(value, `"`, `\"`)
		return fmt.Sprintf(`"%s"`, value)
	}

	return value
}

// ParseLarge parses a large .env file using streaming approach
func ParseLarge(r io.Reader) (*File, error) {
	return ParseLargeWithContext(context.Background(), r)
}

// ParseLargeWithContext parses a large .env file using streaming approach with context
func ParseLargeWithContext(ctx context.Context, r io.Reader) (*File, error) {
	file := NewFile()
	streamer := memory.NewEnvFileStreamer()
	
	result := streamer.StreamParse(ctx, r)
	
	for {
		select {
		case variable, ok := <-result.Variables:
			if !ok {
				// Channel closed, check for completion
				select {
				case <-result.Done:
					return file, nil
				case err := <-result.Errors:
					return nil, err
				}
			}
			
			// Add variable to file
			file.Variables[variable.Key] = &Variable{
				Key:     variable.Key,
				Value:   variable.Value,
				Comment: variable.Comment,
				Line:    variable.Line,
			}
			file.Order = append(file.Order, variable.Key)
			
		case err := <-result.Errors:
			return nil, err
			
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// StreamProcessor provides streaming processing capabilities for env files
type StreamProcessor struct {
	processor *memory.StreamProcessor
}

// NewStreamProcessor creates a new streaming processor
func NewStreamProcessor() *StreamProcessor {
	return &StreamProcessor{
		processor: memory.NewStreamProcessor(8192),
	}
}

// ProcessStream processes an env file stream with a callback function
func (sp *StreamProcessor) ProcessStream(ctx context.Context, r io.Reader, callback func(*Variable) error) error {
	options := memory.StreamOptions{
		BufferSize:  8192,
		MaxLineSize: 64 * 1024,
		LineProcessor: func(line string) error {
			// Skip empty lines and comments
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
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
					value = trimQuotes(value)
					
					variable := &Variable{
						Key:     key,
						Value:   value,
						Comment: comment,
					}
					
					return callback(variable)
				}
			}
			
			return nil
		},
	}
	
	return sp.processor.ProcessReader(ctx, r, options)
}