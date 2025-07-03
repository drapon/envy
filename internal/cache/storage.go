package cache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/drapon/envy/internal/env"
	"github.com/drapon/envy/internal/errors"
	"github.com/drapon/envy/internal/log"
	"go.uber.org/zap"
)

// SerializableCacheEntry はJSON用のキャッシュエントリ構造体
type SerializableCacheEntry struct {
	Key          string                 `json:"key"`
	Value        interface{}            `json:"value"`
	ValueType    string                 `json:"value_type"`
	CreatedAt    time.Time              `json:"created_at"`
	LastAccessed time.Time              `json:"last_accessed"`
	TTL          time.Duration          `json:"ttl"`
	Encrypted    bool                   `json:"encrypted"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Storage はキャッシュストレージのインターフェース
type Storage interface {
	Get(key string) (*CacheEntry, error)
	Set(key string, entry *CacheEntry) error
	Delete(key string) error
	Clear() error
	Cleanup() error
	Close() error
}

// FileStorage はファイルベースのストレージ実装
type FileStorage struct {
	baseDir       string
	encryptionKey []byte
	gcm           cipher.AEAD
	mu            sync.RWMutex
	logger        *zap.Logger
}

// NewFileStorage は新しいファイルストレージを作成
func NewFileStorage(baseDir string, encryptionKey string) (Storage, error) {
	storage := &FileStorage{
		baseDir: baseDir,
		logger:  log.WithContext(zap.String("component", "file_storage")),
	}

	// 暗号化キーが設定されている場合、AES-GCMを初期化
	if encryptionKey != "" {
		keyBytes := sha256.Sum256([]byte(encryptionKey))
		storage.encryptionKey = keyBytes[:]

		block, err := aes.NewCipher(storage.encryptionKey)
		if err != nil {
			return nil, errors.New(errors.ErrInternal, "暗号化の初期化に失敗").WithCause(err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, errors.New(errors.ErrInternal, "GCM暗号化の初期化に失敗").WithCause(err)
		}
		storage.gcm = gcm
	}

	// ベースディレクトリの作成
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, errors.New(errors.ErrFileWrite, "キャッシュディレクトリの作成に失敗").
			WithCause(err).
			WithDetails("base_dir", baseDir)
	}

	storage.logger.Debug("ファイルストレージが初期化されました",
		zap.String("base_dir", baseDir),
		zap.Bool("encryption_enabled", encryptionKey != ""))

	return storage, nil
}

// Get はキーに対応するエントリを取得
func (fs *FileStorage) Get(key string) (*CacheEntry, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filePath := fs.getFilePath(key)
	
	// ファイルの存在確認
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil // エントリが存在しない
	}

	// ファイル読み込み
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.New(errors.ErrFileRead, "キャッシュファイルの読み込みに失敗").
			WithCause(err).
			WithDetails("file_path", filePath)
	}

	// ファイルの権限確認
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, errors.New(errors.ErrFileRead, "ファイル情報の取得に失敗").WithCause(err)
	}
	
	// 権限が適切でない場合は削除
	if fileInfo.Mode().Perm() != 0600 {
		fs.logger.Warn("不適切なファイル権限を検出、削除します",
			zap.String("file_path", filePath),
			zap.String("permissions", fileInfo.Mode().String()))
		os.Remove(filePath)
		return nil, nil
	}

	var entry CacheEntry
	
	// JSON形式のデータを復号化してデシリアライズ
	if err := fs.deserializeEntry(data, &entry); err != nil {
		fs.logger.Warn("キャッシュエントリのデシリアライズに失敗",
			zap.String("key", log.MaskSensitive(key)),
			zap.Error(err))
		// 破損したファイルを削除
		os.Remove(filePath)
		return nil, nil
	}

	fs.logger.Debug("ファイルストレージからエントリを取得",
		zap.String("key", log.MaskSensitive(key)),
		zap.Bool("encrypted", entry.Encrypted))

	return &entry, nil
}

// Set はキーとエントリを保存
func (fs *FileStorage) Set(key string, entry *CacheEntry) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filePath := fs.getFilePath(key)
	
	// ディレクトリの作成
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New(errors.ErrFileWrite, "キャッシュディレクトリの作成に失敗").
			WithCause(err).
			WithDetails("dir", dir)
	}

	// エントリをシリアライズ・暗号化
	data, err := fs.serializeEntry(entry)
	if err != nil {
		return errors.New(errors.ErrInternal, "キャッシュエントリのシリアライズに失敗").
			WithCause(err)
	}

	// 一時ファイルに書き込み、その後原子的にリネーム
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return errors.New(errors.ErrFileWrite, "キャッシュファイルの書き込みに失敗").
			WithCause(err).
			WithDetails("file_path", tempPath)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath) // クリーンアップ
		return errors.New(errors.ErrFileWrite, "キャッシュファイルのリネームに失敗").
			WithCause(err).
			WithDetails("temp_path", tempPath).
			WithDetails("final_path", filePath)
	}

	fs.logger.Debug("ファイルストレージにエントリを保存",
		zap.String("key", log.MaskSensitive(key)),
		zap.String("file_path", filePath),
		zap.Bool("encrypted", entry.Encrypted))

	return nil
}

// Delete はキーに対応するエントリを削除
func (fs *FileStorage) Delete(key string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filePath := fs.getFilePath(key)
	
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return errors.New(errors.ErrFileWrite, "キャッシュファイルの削除に失敗").
			WithCause(err).
			WithDetails("file_path", filePath)
	}

	fs.logger.Debug("ファイルストレージからエントリを削除",
		zap.String("key", log.MaskSensitive(key)),
		zap.String("file_path", filePath))

	return nil
}

// Clear はすべてのキャッシュファイルを削除
func (fs *FileStorage) Clear() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// ベースディレクトリ内のすべてのファイルを削除
	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(path, ".cache") {
			if removeErr := os.Remove(path); removeErr != nil {
				fs.logger.Warn("キャッシュファイルの削除に失敗",
					zap.String("path", path),
					zap.Error(removeErr))
			}
		}
		return nil
	})

	if err != nil {
		return errors.New(errors.ErrFileWrite, "キャッシュディレクトリのクリアに失敗").
			WithCause(err).
			WithDetails("base_dir", fs.baseDir)
	}

	fs.logger.Debug("ファイルストレージをクリアしました",
		zap.String("base_dir", fs.baseDir))

	return nil
}

// Cleanup は期限切れファイルを削除
func (fs *FileStorage) Cleanup() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	removedCount := 0
	now := time.Now()

	err := filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(path, ".cache") {
			// ファイルを読み込んで期限をチェック
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				// 読み込めないファイルは削除
				os.Remove(path)
				removedCount++
				return nil
			}

			var entry CacheEntry
			if deserializeErr := fs.deserializeEntry(data, &entry); deserializeErr != nil {
				// デシリアライズできないファイルは削除
				os.Remove(path)
				removedCount++
				return nil
			}

			// 期限切れの場合は削除
			if entry.TTL > 0 && now.Sub(entry.CreatedAt) > entry.TTL {
				os.Remove(path)
				removedCount++
			}
		}
		return nil
	})

	if err != nil {
		fs.logger.Warn("クリーンアップ中にエラーが発生",
			zap.Error(err))
	}

	if removedCount > 0 {
		fs.logger.Debug("期限切れキャッシュファイルを削除しました",
			zap.Int("removed_count", removedCount))
	}

	return nil
}

// Close はストレージを閉じる
func (fs *FileStorage) Close() error {
	fs.logger.Debug("ファイルストレージを閉じました")
	return nil
}

// getFilePath はキーからファイルパスを生成
func (fs *FileStorage) getFilePath(key string) string {
	// キーをハッシュ化してファイル名として使用
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hashStr := fmt.Sprintf("%x", hasher.Sum(nil))
	
	// ディレクトリ階層を作成（パフォーマンスのため）
	subDir := hashStr[:2]
	fileName := hashStr[2:] + ".cache"
	
	return filepath.Join(fs.baseDir, subDir, fileName)
}

// serializeEntry はエントリをシリアライズ（暗号化含む）
func (fs *FileStorage) serializeEntry(entry *CacheEntry) ([]byte, error) {
	// env.Fileのような特殊な型を扱うためのカスタムシリアライゼーション
	serializable := &SerializableCacheEntry{
		Key:          entry.Key,
		CreatedAt:    entry.CreatedAt,
		LastAccessed: entry.LastAccessed,
		TTL:          entry.TTL,
		Encrypted:    entry.Encrypted,
		Metadata:     entry.Metadata,
	}

	// 値の型をチェックして適切にシリアライズ
	if envFile, ok := entry.Value.(*env.File); ok {
		// env.Fileを特別に扱う
		serializable.ValueType = "*env.File"
		serializable.Value = map[string]interface{}{
			"variables": envFile.ToMap(),
			"order":     envFile.Order,
		}
	} else {
		// その他の型はそのまま
		serializable.ValueType = "generic"
		serializable.Value = entry.Value
	}

	jsonData, err := json.Marshal(serializable)
	if err != nil {
		return nil, fmt.Errorf("JSONシリアライズエラー: %w", err)
	}

	// 暗号化が必要かつ可能な場合
	if entry.Encrypted && fs.gcm != nil {
		return fs.encrypt(jsonData)
	}

	return jsonData, nil
}

// deserializeEntry はデータをデシリアライズ（復号化含む）
func (fs *FileStorage) deserializeEntry(data []byte, entry *CacheEntry) error {
	var jsonData []byte

	// 暗号化データの可能性がある場合、復号化を試行
	if fs.gcm != nil {
		if decrypted, decryptErr := fs.decrypt(data); decryptErr == nil {
			jsonData = decrypted
		} else {
			// 復号化に失敗した場合は非暗号化データとして扱う
			jsonData = data
		}
	} else {
		jsonData = data
	}

	// カスタムデシリアライゼーション
	var serializable SerializableCacheEntry
	if err := json.Unmarshal(jsonData, &serializable); err != nil {
		return fmt.Errorf("JSONデシリアライズエラー: %w", err)
	}

	// CacheEntryに変換
	entry.Key = serializable.Key
	entry.CreatedAt = serializable.CreatedAt
	entry.LastAccessed = serializable.LastAccessed
	entry.TTL = serializable.TTL
	entry.Encrypted = serializable.Encrypted
	entry.Metadata = serializable.Metadata

	// 値の型に応じて復元
	if serializable.ValueType == "*env.File" {
		// env.Fileを復元
		if valueMap, ok := serializable.Value.(map[string]interface{}); ok {
			envFile := env.NewFile()
			
			// variablesを復元
			if variables, ok := valueMap["variables"].(map[string]interface{}); ok {
				for key, value := range variables {
					if strValue, ok := value.(string); ok {
						envFile.Set(key, strValue)
					}
				}
			}
			
			// orderを復元
			if order, ok := valueMap["order"].([]interface{}); ok {
				envFile.Order = make([]string, 0, len(order))
				for _, item := range order {
					if key, ok := item.(string); ok {
						envFile.Order = append(envFile.Order, key)
					}
				}
			}
			
			entry.Value = envFile
		} else {
			return fmt.Errorf("env.Fileの値の復元に失敗")
		}
	} else {
		// その他の型はそのまま
		entry.Value = serializable.Value
	}

	return nil
}

// encrypt はデータを暗号化
func (fs *FileStorage) encrypt(data []byte) ([]byte, error) {
	if fs.gcm == nil {
		return data, nil
	}

	// ランダムなnonceを生成
	nonce := make([]byte, fs.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonceの生成エラー: %w", err)
	}

	// 暗号化
	ciphertext := fs.gcm.Seal(nil, nonce, data, nil)
	
	// nonceと暗号化データを結合
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// decrypt はデータを復号化
func (fs *FileStorage) decrypt(data []byte) ([]byte, error) {
	if fs.gcm == nil {
		return data, nil
	}

	nonceSize := fs.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("データが短すぎます")
	}

	// nonceと暗号化データを分離
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	// 復号化
	plaintext, err := fs.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("復号化に失敗: %w", err)
	}

	return plaintext, nil
}

// CacheManager はキャッシュを管理するヘルパー
type CacheManager struct {
	cache  Cache
	config *CacheConfig
	logger *zap.Logger
}

// NewCacheManager は新しいキャッシュマネージャーを作成
func NewCacheManager(config *CacheConfig) (*CacheManager, error) {
	cache, err := NewCache(config)
	if err != nil {
		return nil, err
	}

	return &CacheManager{
		cache:  cache,
		config: config,
		logger: log.WithContext(zap.String("component", "cache_manager")),
	}, nil
}

// GetOrSet はキャッシュから値を取得、存在しない場合は生成関数を実行
func (cm *CacheManager) GetOrSet(key string, ttl time.Duration, generator func() (interface{}, error)) (interface{}, error) {
	// キャッシュから取得を試行
	if value, found, err := cm.cache.Get(key); err == nil && found {
		return value, nil
	}

	// キャッシュにない場合は生成
	value, err := generator()
	if err != nil {
		return nil, err
	}

	// 生成した値をキャッシュに保存
	if setErr := cm.cache.Set(key, value, ttl); setErr != nil {
		cm.logger.Warn("キャッシュへの保存に失敗",
			zap.String("key", log.MaskSensitive(key)),
			zap.Error(setErr))
	}

	return value, nil
}

// InvalidateByPrefix は指定されたプレフィックスを持つキーをすべて無効化
func (cm *CacheManager) InvalidateByPrefix(prefix string) error {
	// 注意: 現在の実装では全キーの列挙ができないため、
	// 実際のプロダクションでは別途インデックス機能が必要
	cm.logger.Info("プレフィックスによる無効化が要求されました",
		zap.String("prefix", prefix))
	
	// 簡易実装として全キャッシュをクリア
	return cm.cache.Clear()
}

// Stats はキャッシュの統計情報を返す
func (cm *CacheManager) Stats() *CacheStats {
	return cm.cache.Stats()
}

// Clear はキャッシュをクリアする
func (cm *CacheManager) Clear() error {
	return cm.cache.Clear()
}

// Close はキャッシュマネージャーを閉じる  
func (cm *CacheManager) Close() error {
	return cm.cache.Close()
}