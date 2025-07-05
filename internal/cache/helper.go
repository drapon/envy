package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	// グローバルキャッシュマネージャー
	globalManager *CacheManager
)

// InitGlobalCache はグローバルキャッシュを初期化
func InitGlobalCache(v *viper.Viper) error {
	config, err := LoadCacheConfigFromViper(v)
	if err != nil {
		return errors.New(errors.ErrConfigInvalid, "キャッシュ設定の読み込みに失敗").WithCause(err)
	}

	// --no-cache フラグが設定されている場合はキャッシュを無効化
	if v.GetBool("no_cache") {
		config.Enabled = false
		log.Debug("--no-cache フラグによりキャッシュが無効化されました")
	}

	manager, err := NewCacheManager(config)
	if err != nil {
		return errors.New(errors.ErrInternal, "キャッシュマネージャーの初期化に失敗").WithCause(err)
	}

	globalManager = manager

	// --clear-cache フラグが設定されている場合はキャッシュをクリア
	if v.GetBool("clear_cache") {
		if err := manager.cache.Clear(); err != nil {
			log.Warn("キャッシュのクリアに失敗しました", zap.Error(err))
		} else {
			log.Debug("キャッシュをクリアしました")
		}
	}

	return nil
}

// GetGlobalCache はグローバルキャッシュマネージャーを取得
func GetGlobalCache() *CacheManager {
	return globalManager
}

// CloseGlobalCache はグローバルキャッシュを閉じる
func CloseGlobalCache() error {
	if globalManager != nil {
		return globalManager.Close()
	}
	return nil
}

// InitializeGlobalCache initializes the global cache with config
func InitializeGlobalCache(cfg interface{}) error {
	var config *CacheConfig

	// Use reflection to check for Cache field
	if v := reflect.ValueOf(cfg); v.Kind() == reflect.Ptr && v.Elem().Kind() == reflect.Struct {
		if cacheField := v.Elem().FieldByName("Cache"); cacheField.IsValid() {
			if cacheConfig, ok := cacheField.Interface().(CacheConfig); ok {
				config = &cacheConfig
			}
		}
	}

	if config == nil {
		// Use default config
		config = DefaultCacheConfig()
	}

	manager, err := NewCacheManager(config)
	if err != nil {
		return err
	}

	globalManager = manager
	return nil
}

// ShutdownGlobalCache shuts down the global cache
func ShutdownGlobalCache() error {
	return CloseGlobalCache()
}

// LoadCacheConfigFromViper はViperからキャッシュ設定を読み込み
func LoadCacheConfigFromViper(v *viper.Viper) (*CacheConfig, error) {
	config := DefaultCacheConfig()

	// 基本設定
	config.Enabled = v.GetBool("cache.enabled")

	// TTLの解析
	ttlStr := v.GetString("cache.ttl")
	if ttlStr != "" {
		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("無効なTTL形式: %s", ttlStr)
		}
		config.TTL = ttl
	}

	// 最大サイズの解析
	maxSizeStr := v.GetString("cache.max_size")
	if maxSizeStr != "" {
		maxSize, err := parseSizeString(maxSizeStr)
		if err != nil {
			return nil, fmt.Errorf("無効な最大サイズ形式: %s", maxSizeStr)
		}
		config.MaxSize = maxSize
	}

	// 最大エントリ数
	config.MaxEntries = v.GetInt("cache.max_entries")

	// キャッシュディレクトリ
	if cacheDir := v.GetString("cache.dir"); cacheDir != "" {
		config.CacheDir = cacheDir
	} else {
		// デフォルトのキャッシュディレクトリを設定
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("ホームディレクトリの取得に失敗: %w", err)
		}
		config.CacheDir = filepath.Join(homeDir, ".envy", "cache")
	}

	// キャッシュタイプ
	if cacheType := v.GetString("cache.type"); cacheType != "" {
		config.Type = CacheType(cacheType)
	}

	// 暗号化キー（環境変数またはファイルから取得）
	if encKey := v.GetString("cache.encryption_key"); encKey != "" {
		config.EncryptionKey = encKey
	} else if keyFile := v.GetString("cache.encryption_key_file"); keyFile != "" {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("暗号化キーファイルの読み込みに失敗: %w", err)
		}
		config.EncryptionKey = strings.TrimSpace(string(keyData))
	}

	return config, nil
}

// parseSizeString はサイズ文字列（例: "100MB", "1GB"）をバイト数に変換
func parseSizeString(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// 数値部分と単位部分を分離
	var numStr string
	var unit string

	for i, r := range sizeStr {
		if (r >= '0' && r <= '9') || r == '.' {
			numStr += string(r)
		} else {
			unit = sizeStr[i:]
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("無効なサイズ形式: %s", sizeStr)
	}

	size, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("サイズの解析エラー: %w", err)
	}

	// 単位の変換
	switch unit {
	case "", "B":
		return int64(size), nil
	case "KB":
		return int64(size * 1024), nil
	case "MB":
		return int64(size * 1024 * 1024), nil
	case "GB":
		return int64(size * 1024 * 1024 * 1024), nil
	case "TB":
		return int64(size * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("未対応の単位: %s", unit)
	}
}

// CacheKeyBuilder はキャッシュキーを構築するヘルパー
type CacheKeyBuilder struct {
	prefix string
	parts  []string
}

// NewCacheKeyBuilder は新しいキービルダーを作成
func NewCacheKeyBuilder(prefix string) *CacheKeyBuilder {
	return &CacheKeyBuilder{
		prefix: prefix,
		parts:  make([]string, 0),
	}
}

// Add はキーの一部を追加
func (ckb *CacheKeyBuilder) Add(part string) *CacheKeyBuilder {
	ckb.parts = append(ckb.parts, part)
	return ckb
}

// AddF はフォーマット済みの文字列をキーの一部として追加
func (ckb *CacheKeyBuilder) AddF(format string, args ...interface{}) *CacheKeyBuilder {
	ckb.parts = append(ckb.parts, fmt.Sprintf(format, args...))
	return ckb
}

// Build はキーを構築
func (ckb *CacheKeyBuilder) Build() string {
	return GenerateKey(ckb.prefix, ckb.parts...)
}

// CachedOperation はキャッシュ付きの操作を実行
func CachedOperation(key string, ttl time.Duration, operation func() (interface{}, error)) (interface{}, error) {
	if globalManager == nil || !globalManager.config.Enabled {
		return operation()
	}

	return globalManager.GetOrSet(key, ttl, operation)
}

// CachedOperationWithMetadata はメタデータ付きのキャッシュ操作を実行
func CachedOperationWithMetadata(key string, ttl time.Duration, metadata map[string]interface{}, operation func() (interface{}, error)) (interface{}, error) {
	if globalManager == nil || !globalManager.config.Enabled {
		return operation()
	}

	// キャッシュから値を取得を試行
	if value, found, err := globalManager.cache.Get(key); err == nil && found {
		return value, nil
	}

	// キャッシュにない場合は操作を実行
	value, err := operation()
	if err != nil {
		return nil, err
	}

	// メタデータ付きでキャッシュに保存
	if setErr := globalManager.cache.SetWithMetadata(key, value, ttl, metadata); setErr != nil {
		log.Warn("キャッシュへの保存に失敗",
			zap.String("key", log.MaskSensitive(key)),
			zap.Error(setErr))
	}

	return value, nil
}

// InvalidateCache は指定されたキーのキャッシュを無効化
func InvalidateCache(key string) error {
	if globalManager == nil {
		return nil
	}
	return globalManager.cache.Delete(key)
}

// InvalidateCacheByPrefix はプレフィックスに一致するキャッシュを無効化
func InvalidateCacheByPrefix(prefix string) error {
	if globalManager == nil {
		return nil
	}
	return globalManager.InvalidateByPrefix(prefix)
}

// GetCacheStats はキャッシュの統計情報を取得
func GetCacheStats() *CacheStats {
	if globalManager == nil {
		return &CacheStats{}
	}
	return globalManager.Stats()
}

// FormatCacheStats はキャッシュ統計を人間が読める形式でフォーマット
func FormatCacheStats(stats *CacheStats) string {
	var sb strings.Builder

	sb.WriteString("Cache Statistics:\n")
	sb.WriteString(fmt.Sprintf("  Hits: %d\n", stats.Hits))
	sb.WriteString(fmt.Sprintf("  Misses: %d\n", stats.Misses))
	sb.WriteString(fmt.Sprintf("  Hit Rate: %.2f%%\n", stats.HitRate()*100))
	sb.WriteString(fmt.Sprintf("  Entries: %d\n", stats.Entries))
	sb.WriteString(fmt.Sprintf("  Size: %s\n", formatSize(stats.Size)))

	if !stats.LastCleanup.IsZero() {
		sb.WriteString(fmt.Sprintf("  Last Cleanup: %s\n", stats.LastCleanup.Format("2006-01-02 15:04:05")))
	}

	return sb.String()
}

// formatSize はバイト数を人間が読める形式でフォーマット
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FileBasedCacheKey はファイルベースのキャッシュキーを生成
func FileBasedCacheKey(prefix, filePath string) (string, time.Time, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return "", time.Time{}, err
	}

	key := NewCacheKeyBuilder(prefix).
		Add(filePath).
		AddF("mtime:%d", stat.ModTime().Unix()).
		AddF("size:%d", stat.Size()).
		Build()

	return key, stat.ModTime(), nil
}
