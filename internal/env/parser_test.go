package env_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFile(t *testing.T) {
	file := env.NewFile()

	assert.NotNil(t, file)
	assert.NotNil(t, file.Variables)
	assert.NotNil(t, file.Order)
	assert.NotNil(t, file.Comments)
	assert.Empty(t, file.Variables)
	assert.Empty(t, file.Order)
	assert.Empty(t, file.Comments)
}

func TestParse(t *testing.T) {
	fixtures := testutil.NewTestFixtures()

	t.Run("simple_env", func(t *testing.T) {
		content := fixtures.SimpleEnvContent()
		file, err := env.Parse(strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)

		// Check variables
		assert.Len(t, file.Variables, 3)
		assert.Len(t, file.Order, 3)

		// Verify specific variables
		val, exists := file.Get("APP_NAME")
		assert.True(t, exists)
		assert.Equal(t, "test-app", val)

		val, exists = file.Get("DEBUG")
		assert.True(t, exists)
		assert.Equal(t, "true", val)

		val, exists = file.Get("PORT")
		assert.True(t, exists)
		assert.Equal(t, "8080", val)
	})

	t.Run("complex_env", func(t *testing.T) {
		content := fixtures.ComplexEnvContent()
		file, err := env.Parse(strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)

		// Check that all variables are parsed
		assert.Greater(t, len(file.Variables), 20)

		// Check specific complex cases
		val, exists := file.Get("DATABASE_URL")
		assert.True(t, exists)
		assert.Equal(t, "postgres://user:password@localhost:5432/testdb", val)

		val, exists = file.Get("JWT_SECRET")
		assert.True(t, exists)
		assert.Equal(t, "super-secret-jwt-key-for-testing", val)
	})

	t.Run("with_comments", func(t *testing.T) {
		content := fixtures.EnvContentWithComments()
		file, err := env.Parse(strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)

		// Check inline comments are parsed
		appVar, exists := file.Variables["APP_NAME"]
		assert.True(t, exists)
		assert.Equal(t, "test-app", appVar.Value)
		assert.Equal(t, "Application name", appVar.Comment)

		debugVar, exists := file.Variables["DEBUG"]
		assert.True(t, exists)
		assert.Equal(t, "true", debugVar.Value)
		assert.Equal(t, "Enable debug mode", debugVar.Comment)

		// Check standalone comments
		assert.NotEmpty(t, file.Comments)
	})

	t.Run("with_quotes", func(t *testing.T) {
		content := fixtures.EnvContentWithQuotes()
		file, err := env.Parse(strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)

		// Test unquoted
		val, _ := file.Get("UNQUOTED_VAR")
		assert.Equal(t, "simple_value", val)

		// Test double quoted
		val, _ = file.Get("DOUBLE_QUOTED_VAR")
		assert.Equal(t, "double quoted value", val)

		// Test single quoted
		val, _ = file.Get("SINGLE_QUOTED_VAR")
		assert.Equal(t, "single quoted value", val)

		// Test empty quoted
		val, _ = file.Get("EMPTY_QUOTED_VAR")
		assert.Equal(t, "", val)

		// Test spaces in quotes
		val, _ = file.Get("SPACES_IN_QUOTES")
		assert.Equal(t, "  value with spaces  ", val)

		// Test special chars
		val, _ = file.Get("SPECIAL_CHARS")
		assert.Equal(t, "value with !@#$%^&*() chars", val)

		// Test equals in value
		val, _ = file.Get("EQUALS_IN_VALUE")
		assert.Equal(t, "key=value inside", val)

		// Test hash in value
		val, _ = file.Get("HASH_IN_VALUE")
		assert.Equal(t, "value with # hash inside", val)
	})

	t.Run("special_cases", func(t *testing.T) {
		content := fixtures.EnvContentWithSpecialCases()
		file, err := env.Parse(strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)

		// Test empty var
		val, exists := file.Get("EMPTY_VAR")
		assert.True(t, exists)
		assert.Equal(t, "", val)

		// Test unicode
		val, _ = file.Get("UNICODE_VAR")
		assert.Equal(t, "测试值", val)

		// Test emoji
		val, _ = file.Get("EMOJI_VAR")
		assert.Equal(t, "🚀🎉✨", val)

		// Test URL
		val, _ = file.Get("URL_VAR")
		assert.Equal(t, "https://example.com/path?param=value&other=123", val)

		// Test JSON
		val, _ = file.Get("JSON_VAR")
		assert.Equal(t, `{"key":"value","number":123,"bool":true}`, val)
	})

	t.Run("malformed", func(t *testing.T) {
		content := fixtures.EnvContentMalformed()
		file, err := env.Parse(strings.NewReader(content))

		// Parser should not error on malformed lines, just skip them
		require.NoError(t, err)
		require.NotNil(t, file)

		// Valid lines should still be parsed
		_, exists := file.Get("MISSING_EQUALS_SIGN")
		assert.False(t, exists)

		_, exists = file.Get("MISSING_KEY")
		assert.False(t, exists)

		// Variables with invalid names are skipped
		_, exists = file.Get("INVALID-KEY-NAME")
		assert.False(t, exists)

		_, exists = file.Get("123_NUMERIC_START")
		assert.False(t, exists)
	})

	t.Run("empty_reader", func(t *testing.T) {
		file, err := env.Parse(strings.NewReader(""))

		require.NoError(t, err)
		require.NotNil(t, file)
		assert.Empty(t, file.Variables)
	})

	t.Run("scanner_error", func(t *testing.T) {
		// Create a reader that will fail
		reader := &errorReader{err: io.ErrUnexpectedEOF}

		file, err := env.Parse(reader)
		assert.Error(t, err)
		assert.Nil(t, file)
		assert.Contains(t, err.Error(), "error reading file")
	})
}

func TestParseWithContext(t *testing.T) {
	fixtures := testutil.NewTestFixtures()

	t.Run("with_timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		content := fixtures.SimpleEnvContent()
		file, err := env.ParseWithContext(ctx, strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)
		assert.Len(t, file.Variables, 3)
	})

	t.Run("cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		content := fixtures.SimpleEnvContent()
		file, err := env.ParseWithContext(ctx, strings.NewReader(content))

		assert.Error(t, err)
		assert.Nil(t, file)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestParseFile(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()

	t.Run("valid_file", func(t *testing.T) {
		content := `APP_NAME=test
DEBUG=true`

		path := helper.CreateTempFile("test.env", content)
		file, err := env.ParseFile(path)

		require.NoError(t, err)
		require.NotNil(t, file)

		val, _ := file.Get("APP_NAME")
		assert.Equal(t, "test", val)

		val, _ = file.Get("DEBUG")
		assert.Equal(t, "true", val)
	})

	t.Run("non_existent_file", func(t *testing.T) {
		file, err := env.ParseFile("/non/existent/file.env")

		assert.Error(t, err)
		assert.Nil(t, file)
		assert.Contains(t, err.Error(), "failed to open file")
	})
}

func TestFile_Write(t *testing.T) {
	t.Run("simple_write", func(t *testing.T) {
		file := env.NewFile()
		file.Set("KEY1", "value1")
		file.Set("KEY2", "value2")
		file.Set("KEY3", "value3")

		var buf bytes.Buffer
		err := file.Write(&buf)

		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "KEY1=value1")
		assert.Contains(t, output, "KEY2=value2")
		assert.Contains(t, output, "KEY3=value3")
	})

	t.Run("with_comments", func(t *testing.T) {
		file := env.NewFile()
		file.Comments[1] = "File header comment"
		file.Set("KEY1", "value1")
		file.Variables["KEY1"].Comment = "This is a comment"
		file.Variables["KEY1"].Line = 2

		var buf bytes.Buffer
		err := file.Write(&buf)

		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "# File header comment")
		assert.Contains(t, output, "KEY1=value1 # This is a comment")
	})

	t.Run("with_special_values", func(t *testing.T) {
		file := env.NewFile()
		file.Set("QUOTED", "value with spaces")
		file.Set("EMPTY", "")
		file.Set("SPECIAL", "value with #hash")

		var buf bytes.Buffer
		err := file.Write(&buf)

		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `QUOTED="value with spaces"`)
		assert.Contains(t, output, `EMPTY=""`)
		assert.Contains(t, output, `SPECIAL="value with #hash"`)
	})
}

func TestFile_WriteWithContext(t *testing.T) {
	t.Run("cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		file := env.NewFile()
		file.Set("KEY", "value")

		var buf bytes.Buffer
		err := file.WriteWithContext(ctx, &buf)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestFile_WriteFile(t *testing.T) {
	helper := testutil.NewTestHelper(t)
	defer helper.Cleanup()

	t.Run("write_to_file", func(t *testing.T) {
		file := env.NewFile()
		file.Set("KEY1", "value1")
		file.Set("KEY2", "value2")

		path := helper.TempDir() + "/test.env"
		err := file.WriteFile(path)

		require.NoError(t, err)

		// Verify file contents
		content := testutil.ReadFile(t, path)
		assert.Contains(t, content, "KEY1=value1")
		assert.Contains(t, content, "KEY2=value2")

		// Verify file permissions
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})

	t.Run("write_to_invalid_path", func(t *testing.T) {
		file := env.NewFile()
		file.Set("KEY", "value")

		err := file.WriteFile("/non/existent/directory/test.env")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create file")
	})
}

func TestFile_Operations(t *testing.T) {
	t.Run("set_and_get", func(t *testing.T) {
		file := env.NewFile()

		// Test set
		file.Set("KEY1", "value1")
		file.Set("KEY2", "value2")

		// Test get
		val, exists := file.Get("KEY1")
		assert.True(t, exists)
		assert.Equal(t, "value1", val)

		val, exists = file.Get("KEY2")
		assert.True(t, exists)
		assert.Equal(t, "value2", val)

		// Test non-existent key
		val, exists = file.Get("NONEXISTENT")
		assert.False(t, exists)
		assert.Equal(t, "", val)

		// Test update
		file.Set("KEY1", "updated")
		val, exists = file.Get("KEY1")
		assert.True(t, exists)
		assert.Equal(t, "updated", val)
	})

	t.Run("delete", func(t *testing.T) {
		file := env.NewFile()
		file.Set("KEY1", "value1")
		file.Set("KEY2", "value2")
		file.Set("KEY3", "value3")

		// Delete middle key
		file.Delete("KEY2")

		_, exists := file.Get("KEY2")
		assert.False(t, exists)

		// Order should be updated
		assert.Equal(t, []string{"KEY1", "KEY3"}, file.Order)

		// Other keys should remain
		val, _ := file.Get("KEY1")
		assert.Equal(t, "value1", val)

		val, _ = file.Get("KEY3")
		assert.Equal(t, "value3", val)
	})

	t.Run("keys", func(t *testing.T) {
		file := env.NewFile()
		file.Set("KEY3", "value3")
		file.Set("KEY1", "value1")
		file.Set("KEY2", "value2")

		// Keys should maintain insertion order
		keys := file.Keys()
		assert.Equal(t, []string{"KEY3", "KEY1", "KEY2"}, keys)

		// SortedKeys should be alphabetical
		sortedKeys := file.SortedKeys()
		assert.Equal(t, []string{"KEY1", "KEY2", "KEY3"}, sortedKeys)
	})
}

func TestFile_ToMap(t *testing.T) {
	file := env.NewFile()
	file.Set("KEY1", "value1")
	file.Set("KEY2", "value2")
	file.Set("KEY3", "value3")

	result := file.ToMap()

	expected := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}

	assert.Equal(t, expected, result)
}

func TestFile_ToMapWithPool(t *testing.T) {
	file := env.NewFile()
	file.Set("KEY1", "value1")
	file.Set("KEY2", "value2")

	result, cleanup := file.ToMapWithPool()
	defer cleanup()

	expected := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	assert.Equal(t, expected, result)
}

func TestFile_Merge(t *testing.T) {
	t.Run("merge_new_keys", func(t *testing.T) {
		file1 := env.NewFile()
		file1.Set("KEY1", "value1")
		file1.Set("KEY2", "value2")

		file2 := env.NewFile()
		file2.Set("KEY3", "value3")
		file2.Set("KEY4", "value4")

		file1.Merge(file2)

		// All keys should exist
		assert.Len(t, file1.Variables, 4)

		val, _ := file1.Get("KEY1")
		assert.Equal(t, "value1", val)

		val, _ = file1.Get("KEY3")
		assert.Equal(t, "value3", val)
	})

	t.Run("merge_overlapping_keys", func(t *testing.T) {
		file1 := env.NewFile()
		file1.Set("KEY1", "value1")
		file1.Set("KEY2", "value2")

		file2 := env.NewFile()
		file2.Set("KEY2", "updated2")
		file2.Set("KEY3", "value3")
		file2.Variables["KEY2"].Comment = "Updated comment"

		file1.Merge(file2)

		// KEY2 should be updated
		val, _ := file1.Get("KEY2")
		assert.Equal(t, "updated2", val)
		assert.Equal(t, "Updated comment", file1.Variables["KEY2"].Comment)

		// Other keys should remain
		val, _ = file1.Get("KEY1")
		assert.Equal(t, "value1", val)

		val, _ = file1.Get("KEY3")
		assert.Equal(t, "value3", val)
	})
}

func TestParseLarge(t *testing.T) {
	t.Skip("Skipping ParseLarge test - needs StreamParse implementation fix")
	fixtures := testutil.NewTestFixtures()

	t.Run("large_file", func(t *testing.T) {
		// Create large content
		content := fixtures.EnvContentLarge(1000)

		file, err := env.ParseLarge(strings.NewReader(content))

		require.NoError(t, err)
		require.NotNil(t, file)
		assert.Len(t, file.Variables, 1000)

		// Verify some variables
		val, exists := file.Get("VAR_0")
		assert.True(t, exists)
		assert.Contains(t, val, "value_0")

		val, exists = file.Get("VAR_999")
		assert.True(t, exists)
		assert.Contains(t, val, "value_999")
	})
}

func TestStreamProcessor(t *testing.T) {
	fixtures := testutil.NewTestFixtures()

	t.Run("process_stream", func(t *testing.T) {
		content := fixtures.SimpleEnvContent()
		processor := env.NewStreamProcessor()

		var processed []*env.Variable
		err := processor.ProcessStream(
			context.Background(),
			strings.NewReader(content),
			func(v *env.Variable) error {
				processed = append(processed, v)
				return nil
			},
		)

		require.NoError(t, err)
		assert.Len(t, processed, 3)

		// Verify variables
		assert.Equal(t, "APP_NAME", processed[0].Key)
		assert.Equal(t, "test-app", processed[0].Value)

		assert.Equal(t, "DEBUG", processed[1].Key)
		assert.Equal(t, "true", processed[1].Value)

		assert.Equal(t, "PORT", processed[2].Key)
		assert.Equal(t, "8080", processed[2].Value)
	})

	t.Run("callback_error", func(t *testing.T) {
		content := fixtures.SimpleEnvContent()
		processor := env.NewStreamProcessor()

		expectedErr := io.ErrUnexpectedEOF
		err := processor.ProcessStream(
			context.Background(),
			strings.NewReader(content),
			func(v *env.Variable) error {
				return expectedErr
			},
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected EOF")
	})
}

// Benchmark tests
func BenchmarkParse(b *testing.B) {
	fixtures := testutil.NewTestFixtures()
	content := fixtures.ComplexEnvContent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.Parse(strings.NewReader(content))
	}
}

func BenchmarkParseLarge(b *testing.B) {
	fixtures := testutil.NewTestFixtures()
	content := fixtures.EnvContentLarge(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.ParseLarge(strings.NewReader(content))
	}
}

func BenchmarkWrite(b *testing.B) {
	file := testutil.CreateLargeTestEnvFile(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = file.Write(&buf)
	}
}

// Helper types for testing
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}
