package cache

import (
	"testing"
	"time"

	"github.com/drapon/envy/internal/cache"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/log"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCacheCommand(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	t.Run("command_structure", func(t *testing.T) {
		assert.Equal(t, "cache", CacheCmd.Use)
		assert.Equal(t, "Manage cache", CacheCmd.Short)
		assert.NotEmpty(t, CacheCmd.Long)
		assert.NotEmpty(t, CacheCmd.Example)
		assert.NotNil(t, CacheCmd.RunE)
	})

	t.Run("flags", func(t *testing.T) {
		// Check clear flag
		clearFlag := CacheCmd.Flag("clear")
		assert.NotNil(t, clearFlag)
		assert.Equal(t, "bool", clearFlag.Value.Type())

		// Check stats flag
		statsFlag := CacheCmd.Flag("stats")
		assert.NotNil(t, statsFlag)
		assert.Equal(t, "bool", statsFlag.Value.Type())
	})
}

func TestRunCache(t *testing.T) {
	// Initialize test logger
	log.InitLogger(false, "error")

	// Create test config
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Enabled:    true,
			Type:       "memory",
			TTL:        "1h",
			MaxSize:    "10MB",
			MaxEntries: 100,
		},
	}

	// Initialize cache
	cache.InitializeGlobalCache(cfg)
	defer cache.ShutdownGlobalCache()

	t.Run("show_stats_default", func(t *testing.T) {
		cmd := &cobra.Command{}
		clearAll = false
		stats = false

		// Just test that the function runs without error
		err := runCache(cmd, []string{})
		assert.NoError(t, err)
	})

	t.Run("show_stats_flag", func(t *testing.T) {
		cmd := &cobra.Command{}
		clearAll = false
		stats = true

		err := runCache(cmd, []string{})
		assert.NoError(t, err)
	})

	t.Run("clear_cache", func(t *testing.T) {
		// Add some items to cache first
		cacheManager := cache.GetGlobalCache()
		assert.NotNil(t, cacheManager)

		// Add test data
		cacheManager.Set("test_key", "test_value", 1*time.Hour)

		// Verify data exists
		_, found := cacheManager.Get("test_key")
		assert.True(t, found)

		cmd := &cobra.Command{}
		clearAll = true
		stats = false

		err := runCache(cmd, []string{})
		assert.NoError(t, err)

		// Verify cache is cleared
		_, found = cacheManager.Get("test_key")
		assert.False(t, found)
	})
}

func TestClearCache(t *testing.T) {
	// Initialize test logger
	logger := log.InitTestLogger()

	t.Run("cache_not_initialized", func(t *testing.T) {
		// Ensure no global cache
		cache.ShutdownGlobalCache()

		err := clearCache(logger)
		assert.NoError(t, err)
	})

	t.Run("successful_clear", func(t *testing.T) {
		// Initialize cache
		cfg := &config.Config{
			Cache: config.CacheConfig{
				Enabled:    true,
				Type:       "memory",
				TTL:        "1h",
				MaxSize:    "10MB",
				MaxEntries: 100,
			},
		}
		cache.InitializeGlobalCache(cfg)
		defer cache.ShutdownGlobalCache()

		// Add test data
		cacheManager := cache.GetGlobalCache()
		cacheManager.Set("key1", "value1", 1*time.Hour)
		cacheManager.Set("key2", "value2", 1*time.Hour)

		err := clearCache(logger)
		assert.NoError(t, err)
	})
}

func TestShowCacheStats(t *testing.T) {
	// Initialize test logger
	logger := log.InitTestLogger()

	t.Run("cache_not_initialized", func(t *testing.T) {
		// Ensure no global cache
		cache.ShutdownGlobalCache()

		err := showCacheStats(logger)
		assert.NoError(t, err)
	})

	t.Run("show_stats", func(t *testing.T) {
		// Initialize cache
		cfg := &config.Config{
			Cache: config.CacheConfig{
				Enabled:    true,
				Type:       "memory",
				TTL:        "1h",
				MaxSize:    "10MB",
				MaxEntries: 100,
			},
		}
		cache.InitializeGlobalCache(cfg)
		defer cache.ShutdownGlobalCache()

		// Add test data and access it
		cacheManager := cache.GetGlobalCache()
		cacheManager.Set("key1", "value1", 1*time.Hour)
		cacheManager.Get("key1") // Hit
		cacheManager.Get("key2") // Miss

		err := showCacheStats(logger)
		assert.NoError(t, err)
	})
}

func TestFormatSize(t *testing.T) {
	testCases := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    2048,
			expected: "2.00 KB",
		},
		{
			name:     "megabytes",
			bytes:    5 * 1024 * 1024,
			expected: "5.00 MB",
		},
		{
			name:     "gigabytes",
			bytes:    2 * 1024 * 1024 * 1024,
			expected: "2.00 GB",
		},
		{
			name:     "terabytes",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.00 TB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatSize(tc.bytes)
			assert.Equal(t, tc.expected, result)
		})
	}
}
