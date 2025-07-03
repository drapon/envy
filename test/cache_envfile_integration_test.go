package test

import (
	"context"
	"testing"
	"time"

	"github.com/drapon/envy/internal/cache"
	"github.com/drapon/envy/internal/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheEnvFileIntegration(t *testing.T) {
	// Cache configuration
	config := &cache.CacheConfig{
		Type:       cache.DiskCache,
		TTL:        time.Hour,
		MaxSize:    10 * 1024 * 1024,
		MaxEntries: 100,
		Enabled:    true,
	}

	// Create cache manager
	manager, err := cache.NewCacheManager(config)
	require.NoError(t, err)
	defer manager.Close()

	t.Run("CachedOperationWithMetadata processes env.File correctly", func(t *testing.T) {
		// Cache key
		cacheKey := "test-env-file-operation"

		// First call (cache miss)
		result1, err := manager.GetOrSet(
			cacheKey,
			15*time.Minute,
			func() (interface{}, error) {
				// Simulate fetching from AWS
				envFile := env.NewFile()
				envFile.Set("DATABASE_URL", "postgres://localhost:5432/test")
				envFile.Set("REDIS_URL", "redis://localhost:6379")
				envFile.Set("API_KEY", "test-api-key")
				return envFile, nil
			},
		)

		require.NoError(t, err)
		envFile1, ok := result1.(*env.File)
		assert.True(t, ok, "result should be *env.File")
		assert.NotNil(t, envFile1)

		// Verify values
		val, exists := envFile1.Get("DATABASE_URL")
		assert.True(t, exists)
		assert.Equal(t, "postgres://localhost:5432/test", val)

		// Second call (cache hit)
		result2, err := manager.GetOrSet(
			cacheKey,
			15*time.Minute,
			func() (interface{}, error) {
				// This function should not be called
				t.Fatal("generator function was called (should be cache hit)")
				return nil, nil
			},
		)

		require.NoError(t, err)
		envFile2, ok := result2.(*env.File)
		assert.True(t, ok, "cached result should also be *env.File")
		assert.NotNil(t, envFile2)

		// Verify that values restored from cache are correct
		val, exists = envFile2.Get("DATABASE_URL")
		assert.True(t, exists)
		assert.Equal(t, "postgres://localhost:5432/test", val)

		val, exists = envFile2.Get("REDIS_URL")
		assert.True(t, exists)
		assert.Equal(t, "redis://localhost:6379", val)

		val, exists = envFile2.Get("API_KEY")
		assert.True(t, exists)
		assert.Equal(t, "test-api-key", val)
	})

	t.Run("simulate pull command", func(t *testing.T) {
		// Simulate AWS manager's PullEnvironment
		pullEnvironmentWithCache := func(ctx context.Context, envName string) (*env.File, error) {
			cacheKey := cache.NewCacheKeyBuilder("aws_env").
				Add(envName).
				Add("us-east-1").
				Add("/myapp/production/").
				Build()

			result, err := manager.GetOrSet(
				cacheKey,
				15*time.Minute,
				func() (interface{}, error) {
					// Simulate fetching from AWS
					envFile := env.NewFile()
					envFile.Set("APP_ENV", "production")
					envFile.Set("LOG_LEVEL", "info")
					envFile.Set("SECRET_KEY", "super-secret-key")
					return envFile, nil
				},
			)

			if err != nil {
				return nil, err
			}

			envFile, ok := result.(*env.File)
			if !ok {
				t.Fatal("invalid cached environment file type")
			}

			return envFile, nil
		}

		// First call
		envFile1, err := pullEnvironmentWithCache(context.Background(), "production")
		require.NoError(t, err)
		assert.NotNil(t, envFile1)

		// Second call (from cache)
		envFile2, err := pullEnvironmentWithCache(context.Background(), "production")
		require.NoError(t, err)
		assert.NotNil(t, envFile2)

		// Verify that both results have the same values
		val1, _ := envFile1.Get("APP_ENV")
		val2, _ := envFile2.Get("APP_ENV")
		assert.Equal(t, val1, val2)
	})
}
