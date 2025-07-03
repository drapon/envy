package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drapon/envy/internal/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_EnvFile(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "envy-cache-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// ストレージを初期化
	storage, err := NewFileStorage(tempDir, "")
	require.NoError(t, err)
	defer storage.Close()

	t.Run("env.File の保存と復元", func(t *testing.T) {
		// テスト用のenv.Fileを作成
		envFile := env.NewFile()
		envFile.Set("DATABASE_URL", "postgres://localhost:5432/test")
		envFile.Set("API_KEY", "test-api-key-123")
		envFile.Set("DEBUG", "true")
		
		// 元のOrderを保持
		expectedOrder := []string{"DATABASE_URL", "API_KEY", "DEBUG"}
		envFile.Order = expectedOrder

		// キャッシュエントリを作成
		entry := &CacheEntry{
			Key:          "test-env-file",
			Value:        envFile,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			TTL:          time.Hour,
			Metadata: map[string]interface{}{
				"type":        "aws_environment",
				"environment": "test",
			},
		}

		// ストレージに保存
		err := storage.Set("test-env-file", entry)
		assert.NoError(t, err)

		// ストレージから復元
		restored, err := storage.Get("test-env-file")
		assert.NoError(t, err)
		assert.NotNil(t, restored)

		// 復元された値がenv.Fileであることを確認
		restoredEnvFile, ok := restored.Value.(*env.File)
		assert.True(t, ok, "復元された値は*env.Fileであるべき")
		assert.NotNil(t, restoredEnvFile)

		// 値が正しく復元されているか確認
		val, exists := restoredEnvFile.Get("DATABASE_URL")
		assert.True(t, exists)
		assert.Equal(t, "postgres://localhost:5432/test", val)

		val, exists = restoredEnvFile.Get("API_KEY")
		assert.True(t, exists)
		assert.Equal(t, "test-api-key-123", val)

		val, exists = restoredEnvFile.Get("DEBUG")
		assert.True(t, exists)
		assert.Equal(t, "true", val)

		// Orderが保持されているか確認
		assert.Equal(t, expectedOrder, restoredEnvFile.Order)
	})

	t.Run("暗号化されたenv.File", func(t *testing.T) {
		// 暗号化キー付きストレージを作成
		encryptedStorage, err := NewFileStorage(filepath.Join(tempDir, "encrypted"), "test-encryption-key")
		require.NoError(t, err)
		defer encryptedStorage.Close()

		// テスト用のenv.Fileを作成
		envFile := env.NewFile()
		envFile.Set("SECRET_KEY", "super-secret-value")
		envFile.Set("API_TOKEN", "bearer-token-xyz")

		// キャッシュエントリを作成（暗号化フラグ付き）
		entry := &CacheEntry{
			Key:          "encrypted-env-file",
			Value:        envFile,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			TTL:          time.Hour,
			Encrypted:    true,
			Metadata: map[string]interface{}{
				"sensitive": true,
			},
		}

		// ストレージに保存
		err = encryptedStorage.Set("encrypted-env-file", entry)
		assert.NoError(t, err)

		// ストレージから復元
		restored, err := encryptedStorage.Get("encrypted-env-file")
		assert.NoError(t, err)
		assert.NotNil(t, restored)

		// 復元された値がenv.Fileであることを確認
		restoredEnvFile, ok := restored.Value.(*env.File)
		assert.True(t, ok, "復元された値は*env.Fileであるべき")

		// 値が正しく復元されているか確認
		val, exists := restoredEnvFile.Get("SECRET_KEY")
		assert.True(t, exists)
		assert.Equal(t, "super-secret-value", val)

		val, exists = restoredEnvFile.Get("API_TOKEN")
		assert.True(t, exists)
		assert.Equal(t, "bearer-token-xyz", val)
	})

	t.Run("他の型も引き続き動作する", func(t *testing.T) {
		// 通常の文字列値
		entry := &CacheEntry{
			Key:          "simple-string",
			Value:        "test-value",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			TTL:          time.Hour,
		}

		err := storage.Set("simple-string", entry)
		assert.NoError(t, err)

		restored, err := storage.Get("simple-string")
		assert.NoError(t, err)
		assert.NotNil(t, restored)
		assert.Equal(t, "test-value", restored.Value)
	})
}