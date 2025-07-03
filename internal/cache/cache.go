package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"go.uber.org/zap"
)

// CacheEntry はキャッシュエントリの構造体
type CacheEntry struct {
	Key         string                 `json:"key"`
	Value       interface{}            `json:"value"`
	CreatedAt   time.Time              `json:"created_at"`
	LastAccessed time.Time             `json:"last_accessed"`
	TTL         time.Duration          `json:"ttl"`
	Encrypted   bool                   `json:"encrypted"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// IsExpired はエントリが期限切れかどうかを判定
func (e *CacheEntry) IsExpired() bool {
	if e.TTL == 0 {
		return false // TTLが0の場合は期限なし
	}
	return time.Since(e.CreatedAt) > e.TTL
}

// CacheType はキャッシュの種類を表す
type CacheType string

const (
	// MemoryCache メモリ内キャッシュ
	MemoryCache CacheType = "memory"
	// DiskCache ディスクベースキャッシュ
	DiskCache CacheType = "disk"
	// HybridCache メモリ＋ディスクキャッシュ
	HybridCache CacheType = "hybrid"
)

// CacheConfig はキャッシュの設定
type CacheConfig struct {
	Type        CacheType     `json:"type"`
	TTL         time.Duration `json:"ttl"`
	MaxSize     int64         `json:"max_size"`     // バイト単位
	MaxEntries  int           `json:"max_entries"`  // エントリ数
	CacheDir    string        `json:"cache_dir"`    // キャッシュディレクトリ
	EncryptionKey string      `json:"-"`            // 暗号化キー（JSONには含めない）
	Enabled     bool          `json:"enabled"`
}

// DefaultCacheConfig はデフォルトの設定を返す
func DefaultCacheConfig() *CacheConfig {
	homeDir, _ := os.UserHomeDir()
	return &CacheConfig{
		Type:       HybridCache,
		TTL:        time.Hour,
		MaxSize:    100 * 1024 * 1024, // 100MB
		MaxEntries: 1000,
		CacheDir:   filepath.Join(homeDir, ".envy", "cache"),
		Enabled:    true,
	}
}

// Cache はキャッシュのインターフェース
type Cache interface {
	Get(key string) (interface{}, bool, error)
	Set(key string, value interface{}, ttl time.Duration) error
	SetWithMetadata(key string, value interface{}, ttl time.Duration, metadata map[string]interface{}) error
	Delete(key string) error
	Clear() error
	Stats() *CacheStats
	Close() error
}

// CacheStats はキャッシュの統計情報
type CacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Entries     int       `json:"entries"`
	Size        int64     `json:"size"`
	LastCleanup time.Time `json:"last_cleanup"`
}

// HitRate はヒット率を計算
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0.0
	}
	return float64(s.Hits) / float64(total)
}

// cacheImpl はキャッシュの実装
type cacheImpl struct {
	config   *CacheConfig
	memory   map[string]*CacheEntry
	storage  Storage
	mu       sync.RWMutex
	stats    *CacheStats
	stopCh   chan struct{}
	logger   *zap.Logger
	closed   bool
	closeOnce sync.Once
}

// NewCache は新しいキャッシュインスタンスを作成
func NewCache(config *CacheConfig) (Cache, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	// CacheDirが空の場合はデフォルト値を設定
	if config.CacheDir == "" {
		config.CacheDir = DefaultCacheConfig().CacheDir
	}

	// メモリキャッシュの場合はディレクトリ作成をスキップ
	if config.Type == DiskCache || config.Type == HybridCache {
		// キャッシュディレクトリの作成
		if err := os.MkdirAll(config.CacheDir, 0700); err != nil {
			return nil, errors.New(errors.ErrFileWrite, "キャッシュディレクトリの作成に失敗").
				WithCause(err).
				WithDetails("cache_dir", config.CacheDir)
		}
	}

	// ストレージの初期化
	var storage Storage
	var err error
	
	if config.Type == DiskCache || config.Type == HybridCache {
		storage, err = NewFileStorage(config.CacheDir, config.EncryptionKey)
		if err != nil {
			return nil, errors.New(errors.ErrInternal, "ストレージの初期化に失敗").
				WithCause(err)
		}
	}

	cache := &cacheImpl{
		config:  config,
		memory:  make(map[string]*CacheEntry),
		storage: storage,
		stats:   &CacheStats{},
		stopCh:  make(chan struct{}),
		logger:  log.WithContext(zap.String("component", "cache")),
	}

	// 定期的なクリーンアップを開始
	go cache.cleanupRoutine()

	cache.logger.Debug("キャッシュが初期化されました",
		zap.String("type", string(config.Type)),
		zap.Duration("ttl", config.TTL),
		zap.Int64("max_size", config.MaxSize),
		zap.String("cache_dir", config.CacheDir),
	)

	return cache, nil
}

// Get はキーに対応する値を取得
func (c *cacheImpl) Get(key string) (interface{}, bool, error) {
	if !c.config.Enabled {
		return nil, false, nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// メモリキャッシュから検索
	if entry, exists := c.memory[key]; exists {
		if entry.IsExpired() {
			c.logger.Debug("メモリキャッシュエントリが期限切れ", zap.String("key", c.maskKey(key)))
			delete(c.memory, key)
			c.stats.Misses++
			return nil, false, nil
		}
		
		// アクセス時間を更新
		entry.LastAccessed = time.Now()
		c.stats.Hits++
		
		c.logger.Debug("メモリキャッシュヒット", zap.String("key", c.maskKey(key)))
		return entry.Value, true, nil
	}

	// ディスクキャッシュから検索（HybridCacheまたはDiskCacheの場合）
	if c.storage != nil {
		entry, err := c.storage.Get(key)
		if err != nil {
			c.logger.Debug("ディスクキャッシュ読み込みエラー",
				zap.String("key", c.maskKey(key)),
				zap.Error(err))
			c.stats.Misses++
			return nil, false, nil
		}

		if entry != nil && !entry.IsExpired() {
			// メモリキャッシュにも保存（HybridCacheの場合）
			if c.config.Type == HybridCache {
				c.memory[key] = entry
			}
			
			entry.LastAccessed = time.Now()
			c.stats.Hits++
			
			c.logger.Debug("ディスクキャッシュヒット", zap.String("key", c.maskKey(key)))
			return entry.Value, true, nil
		}
	}

	c.stats.Misses++
	return nil, false, nil
}

// Set はキーと値をキャッシュに保存
func (c *cacheImpl) Set(key string, value interface{}, ttl time.Duration) error {
	return c.SetWithMetadata(key, value, ttl, nil)
}

// SetWithMetadata はメタデータ付きでキーと値をキャッシュに保存
func (c *cacheImpl) SetWithMetadata(key string, value interface{}, ttl time.Duration, metadata map[string]interface{}) error {
	if !c.config.Enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// TTLのデフォルト値設定
	if ttl == 0 {
		ttl = c.config.TTL
	}

	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		CreatedAt:   time.Now(),
		LastAccessed: time.Now(),
		TTL:         ttl,
		Metadata:    metadata,
	}

	// センシティブデータの判定
	if c.isSensitiveData(key, value, metadata) {
		entry.Encrypted = true
	}

	// メモリキャッシュに保存
	if c.config.Type == MemoryCache || c.config.Type == HybridCache {
		// メモリ使用量のチェック
		if c.shouldEvictFromMemory() {
			c.evictLRU()
		}
		c.memory[key] = entry
	}

	// ディスクキャッシュに保存
	if c.storage != nil {
		if err := c.storage.Set(key, entry); err != nil {
			c.logger.Warn("ディスクキャッシュ保存エラー",
				zap.String("key", c.maskKey(key)),
				zap.Error(err))
			// メモリキャッシュは成功しているので、エラーとしない
		}
	}

	c.logger.Debug("キャッシュに保存しました",
		zap.String("key", c.maskKey(key)),
		zap.Duration("ttl", ttl),
		zap.Bool("encrypted", entry.Encrypted))

	return nil
}

// Delete はキーに対応するエントリを削除
func (c *cacheImpl) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// メモリキャッシュから削除
	delete(c.memory, key)

	// ディスクキャッシュから削除
	if c.storage != nil {
		if err := c.storage.Delete(key); err != nil {
			c.logger.Warn("ディスクキャッシュ削除エラー",
				zap.String("key", c.maskKey(key)),
				zap.Error(err))
		}
	}

	c.logger.Debug("キャッシュから削除しました", zap.String("key", c.maskKey(key)))
	return nil
}

// Clear はすべてのキャッシュエントリを削除
func (c *cacheImpl) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// メモリキャッシュをクリア
	c.memory = make(map[string]*CacheEntry)

	// ディスクキャッシュをクリア
	if c.storage != nil {
		if err := c.storage.Clear(); err != nil {
			c.logger.Warn("ディスクキャッシュクリアエラー", zap.Error(err))
		}
	}

	// 統計をリセット
	c.stats = &CacheStats{}

	c.logger.Debug("キャッシュをクリアしました")
	return nil
}

// Stats はキャッシュの統計情報を返す
func (c *cacheImpl) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := *c.stats // コピーを作成
	stats.Entries = len(c.memory)
	
	// メモリ使用量を計算（概算）
	for _, entry := range c.memory {
		stats.Size += c.estimateSize(entry)
	}

	return &stats
}

// Close はキャッシュを閉じる
func (c *cacheImpl) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.stopCh)

		c.mu.Lock()
		defer c.mu.Unlock()

		c.closed = true
		if c.storage != nil {
			err = c.storage.Close()
		}

		c.logger.Debug("キャッシュを閉じました")
	})
	return err
}

// cleanupRoutine は定期的なクリーンアップを実行
func (c *cacheImpl) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute) // 10分ごとにクリーンアップ
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup は期限切れエントリを削除
func (c *cacheImpl) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// 期限切れエントリを特定
	for key, entry := range c.memory {
		if entry.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 期限切れエントリを削除
	for _, key := range expiredKeys {
		delete(c.memory, key)
	}

	// ディスクキャッシュのクリーンアップ
	if c.storage != nil {
		c.storage.Cleanup()
	}

	c.stats.LastCleanup = now

	if len(expiredKeys) > 0 {
		c.logger.Debug("期限切れエントリをクリーンアップしました",
			zap.Int("count", len(expiredKeys)))
	}
}

// evictLRU はLRU（Least Recently Used）戦略でエントリを削除
func (c *cacheImpl) evictLRU() {
	if len(c.memory) == 0 {
		return
	}

	// 最も古いアクセス時間のエントリを見つける
	var oldestKey string
	var oldestTime time.Time
	
	for key, entry := range c.memory {
		if oldestKey == "" || entry.LastAccessed.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccessed
		}
	}

	if oldestKey != "" {
		delete(c.memory, oldestKey)
		c.logger.Debug("LRU削除を実行しました", zap.String("key", c.maskKey(oldestKey)))
	}
}

// shouldEvictFromMemory はメモリからの削除が必要かどうかを判定
func (c *cacheImpl) shouldEvictFromMemory() bool {
	if c.config.MaxEntries > 0 && len(c.memory) >= c.config.MaxEntries {
		return true
	}

	// メモリ使用量の簡易チェック
	if c.config.MaxSize > 0 {
		totalSize := int64(0)
		for _, entry := range c.memory {
			totalSize += c.estimateSize(entry)
		}
		return totalSize >= c.config.MaxSize
	}

	return false
}

// isSensitiveData はセンシティブなデータかどうかを判定
func (c *cacheImpl) isSensitiveData(key string, value interface{}, metadata map[string]interface{}) bool {
	// キーにセンシティブなパターンが含まれているかチェック
	sensitivePatterns := []string{
		"password", "secret", "token", "key", "credential",
		"パスワード", "秘密", "トークン", "キー", "認証",
	}
	
	keyLower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}

	// メタデータでセンシティブ指定があるかチェック
	if metadata != nil {
		if sensitive, ok := metadata["sensitive"].(bool); ok && sensitive {
			return true
		}
	}

	return false
}

// estimateSize はエントリのサイズを概算
func (c *cacheImpl) estimateSize(entry *CacheEntry) int64 {
	// 簡易的なサイズ計算
	size := int64(len(entry.Key))
	
	// 値のサイズ（文字列の場合）
	if str, ok := entry.Value.(string); ok {
		size += int64(len(str))
	} else {
		size += 100 // その他の型の場合は概算
	}
	
	return size
}

// maskKey はログ出力用にキーをマスク
func (c *cacheImpl) maskKey(key string) string {
	return log.MaskSensitive(key)
}

// GenerateKey はキャッシュキーを生成
func GenerateKey(prefix string, parts ...string) string {
	hasher := sha256.New()
	hasher.Write([]byte(prefix))
	for _, part := range parts {
		hasher.Write([]byte(part))
	}
	return hex.EncodeToString(hasher.Sum(nil))[:16] // 16文字に短縮
}

// IsFileModified はファイルの更新を検知
func IsFileModified(filePath string, cachedTime time.Time) bool {
	stat, err := os.Stat(filePath)
	if err != nil {
		return true // ファイルが存在しない場合は更新されたとみなす
	}
	return stat.ModTime().After(cachedTime)
}