package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		entry    *CacheEntry
		expected bool
	}{
		{
			name: "期限切れでない（TTL=0）",
			entry: &CacheEntry{
				CreatedAt: time.Now().Add(-1 * time.Hour),
				TTL:       0, // TTL=0は期限なし
			},
			expected: false,
		},
		{
			name: "期限切れでない",
			entry: &CacheEntry{
				CreatedAt: time.Now().Add(-30 * time.Minute),
				TTL:       1 * time.Hour,
			},
			expected: false,
		},
		{
			name: "期限切れ",
			entry: &CacheEntry{
				CreatedAt: time.Now().Add(-2 * time.Hour),
				TTL:       1 * time.Hour,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.IsExpired()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMemoryCache(t *testing.T) {
	config := &CacheConfig{
		Type:       MemoryCache,
		TTL:        1 * time.Hour,
		MaxSize:    1024 * 1024,
		MaxEntries: 100,
		Enabled:    true,
	}

	cache, err := NewCache(config)
	require.NoError(t, err)
	defer cache.Close()

	// Set/Get テスト
	t.Run("Set and Get", func(t *testing.T) {
		key := "test_key"
		value := "test_value"

		err := cache.Set(key, value, 0)
		require.NoError(t, err)

		result, found, err := cache.Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})

	// Delete テスト
	t.Run("Delete", func(t *testing.T) {
		key := "delete_key"
		value := "delete_value"

		err := cache.Set(key, value, 0)
		require.NoError(t, err)

		err = cache.Delete(key)
		require.NoError(t, err)

		_, found, err := cache.Get(key)
		require.NoError(t, err)
		assert.False(t, found)
	})

	// TTL テスト
	t.Run("TTL Expiration", func(t *testing.T) {
		key := "ttl_key"
		value := "ttl_value"

		err := cache.Set(key, value, 100*time.Millisecond)
		require.NoError(t, err)

		// 即座に取得可能
		result, found, err := cache.Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)

		// TTL経過後は取得不可
		time.Sleep(150 * time.Millisecond)
		_, found, err = cache.Get(key)
		require.NoError(t, err)
		assert.False(t, found)
	})

	// Clear テスト
	t.Run("Clear", func(t *testing.T) {
		keys := []string{"clear1", "clear2", "clear3"}
		for _, key := range keys {
			err := cache.Set(key, "value", 0)
			require.NoError(t, err)
		}

		err := cache.Clear()
		require.NoError(t, err)

		for _, key := range keys {
			_, found, err := cache.Get(key)
			require.NoError(t, err)
			assert.False(t, found)
		}
	})
}

func TestDiskCache(t *testing.T) {
	tmpDir := t.TempDir()

	config := &CacheConfig{
		Type:       DiskCache,
		TTL:        1 * time.Hour,
		MaxSize:    1024 * 1024,
		MaxEntries: 100,
		CacheDir:   tmpDir,
		Enabled:    true,
	}

	cache, err := NewCache(config)
	require.NoError(t, err)
	defer cache.Close()

	// ディスクキャッシュのテスト
	t.Run("Disk Persistence", func(t *testing.T) {
		key := "disk_key"
		value := "disk_value"

		err := cache.Set(key, value, 0)
		require.NoError(t, err)

		// キャッシュを再作成（メモリクリア）
		cache.Close()
		newCache, err := NewCache(config)
		require.NoError(t, err)
		defer newCache.Close()

		// ディスクから読み込めることを確認
		result, found, err := newCache.Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})
}

func TestEncryptedCache(t *testing.T) {
	tmpDir := t.TempDir()

	config := &CacheConfig{
		Type:          HybridCache,
		TTL:           1 * time.Hour,
		MaxSize:       1024 * 1024,
		MaxEntries:    100,
		CacheDir:      tmpDir,
		EncryptionKey: "test-encryption-key-32-characters",
		Enabled:       true,
	}

	cache, err := NewCache(config)
	require.NoError(t, err)
	defer cache.Close()

	// センシティブデータの暗号化テスト
	t.Run("Sensitive Data Encryption", func(t *testing.T) {
		key := "password_key"
		value := "secret_password"
		metadata := map[string]interface{}{
			"sensitive": true,
		}

		err := cache.SetWithMetadata(key, value, 0, metadata)
		require.NoError(t, err)

		// 値を取得できることを確認
		result, found, err := cache.Get(key)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, value, result)
	})
}

func TestCacheStats(t *testing.T) {
	config := &CacheConfig{
		Type:       MemoryCache,
		TTL:        1 * time.Hour,
		MaxSize:    1024 * 1024,
		MaxEntries: 100,
		Enabled:    true,
	}

	cache, err := NewCache(config)
	require.NoError(t, err)
	defer cache.Close()

	// 統計情報のテスト
	t.Run("Stats Tracking", func(t *testing.T) {
		// ヒットとミスの記録
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		// ヒット
		cache.Get("key1")
		cache.Get("key2")

		// ミス
		cache.Get("nonexistent")

		stats := cache.Stats()
		assert.Equal(t, int64(2), stats.Hits)
		assert.Equal(t, int64(1), stats.Misses)
		assert.Equal(t, 2, stats.Entries)
		assert.InDelta(t, 0.67, stats.HitRate(), 0.01)
	})
}

func TestCacheHelper(t *testing.T) {
	// ヘルパー関数のテスト
	t.Run("GenerateKey", func(t *testing.T) {
		key1 := GenerateKey("prefix", "part1", "part2")
		key2 := GenerateKey("prefix", "part1", "part2")
		key3 := GenerateKey("prefix", "part1", "part3")

		// 同じ引数からは同じキーが生成される
		assert.Equal(t, key1, key2)
		// 異なる引数からは異なるキーが生成される
		assert.NotEqual(t, key1, key3)
		// キーは適切な長さ
		assert.Len(t, key1, 16)
	})

	t.Run("IsFileModified", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "test.txt")

		// ファイルを作成
		err := os.WriteFile(tmpFile, []byte("content"), 0644)
		require.NoError(t, err)

		stat, err := os.Stat(tmpFile)
		require.NoError(t, err)

		// 同じ時刻では変更されていない
		assert.False(t, IsFileModified(tmpFile, stat.ModTime()))

		// 過去の時刻と比較すると変更されている
		pastTime := stat.ModTime().Add(-1 * time.Hour)
		assert.True(t, IsFileModified(tmpFile, pastTime))

		// 存在しないファイルは変更されたとみなす
		assert.True(t, IsFileModified("/nonexistent", time.Now()))
	})
}

func TestCacheKeyBuilder(t *testing.T) {
	t.Run("CacheKeyBuilder", func(t *testing.T) {
		builder := NewCacheKeyBuilder("prefix")
		key := builder.
			Add("part1").
			Add("part2").
			AddF("formatted_%d", 123).
			Build()

		assert.NotEmpty(t, key)
		assert.Len(t, key, 16)

		// 同じ構築手順で同じキーが生成される
		key2 := NewCacheKeyBuilder("prefix").
			Add("part1").
			Add("part2").
			AddF("formatted_%d", 123).
			Build()

		assert.Equal(t, key, key2)
	})
}

func TestCacheManager(t *testing.T) {
	config := &CacheConfig{
		Type:       MemoryCache,
		TTL:        1 * time.Hour,
		MaxSize:    1024 * 1024,
		MaxEntries: 100,
		Enabled:    true,
	}

	manager, err := NewCacheManager(config)
	require.NoError(t, err)
	defer manager.Close()

	t.Run("GetOrSet", func(t *testing.T) {
		key := "getorset_key"
		expectedValue := "generated_value"
		callCount := 0

		generator := func() (interface{}, error) {
			callCount++
			return expectedValue, nil
		}

		// 最初の呼び出しではgeneratorが実行される
		result, err := manager.GetOrSet(key, 0, generator)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, result)
		assert.Equal(t, 1, callCount)

		// 2回目の呼び出しではキャッシュから取得される
		result, err = manager.GetOrSet(key, 0, generator)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, result)
		assert.Equal(t, 1, callCount) // generatorは実行されない
	})
}
