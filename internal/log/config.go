package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Constants for log configuration.
const (
	OutputFile = "file"
	FormatJSON = "json"
)

// Config はログシステムの設定を保持します.
type Config struct {
	// Level ログレベル (debug, info, warn, error)
	Level LogLevel `mapstructure:"level"`

	// Format ログフォーマット (json, console)
	Format string `mapstructure:"format"`

	// Output 出力先 (stdout, file, syslog)
	Output string `mapstructure:"output"`

	// FilePath ファイル出力時のパス
	FilePath string `mapstructure:"file_path"`

	// Development 開発モード（より詳細なログ）
	Development bool `mapstructure:"development"`

	// EnableCaller 呼び出し元の情報を含める
	EnableCaller bool `mapstructure:"enable_caller"`

	// EnableStacktrace エラー時のスタックトレースを含める
	EnableStacktrace bool `mapstructure:"enable_stacktrace"`

	// MaxSize ログファイルの最大サイズ（MB）
	MaxSize int `mapstructure:"max_size"`

	// MaxBackups 保持する古いログファイルの最大数
	MaxBackups int `mapstructure:"max_backups"`

	// MaxAge 古いログファイルを保持する最大日数
	MaxAge int `mapstructure:"max_age"`

	// Compress 古いログファイルを圧縮するか
	Compress bool `mapstructure:"compress"`

	// SensitiveKeys センシティブな情報として扱うキーのリスト
	SensitiveKeys []string `mapstructure:"sensitive_keys"`
}

// DefaultConfig はデフォルトのログ設定を返します.
func DefaultConfig() *Config {
	return &Config{
		Level:            InfoLevel,
		Format:           "console",
		Output:           "stdout",
		Development:      false,
		EnableCaller:     false,
		EnableStacktrace: false,
		MaxSize:          100,
		MaxBackups:       3,
		MaxAge:           28,
		Compress:         false,
		SensitiveKeys: []string{
			"password",
			"secret",
			"token",
			"key",
			"credential",
			"aws_access_key_id",
			"aws_secret_access_key",
		},
	}
}

// DevelopmentConfig は開発環境用のログ設定を返します.
func DevelopmentConfig() *Config {
	config := DefaultConfig()
	config.Level = DebugLevel
	config.Development = true
	config.EnableCaller = true
	config.EnableStacktrace = true
	return config
}

// ProductionConfig は本番環境用のログ設定を返します.
func ProductionConfig() *Config {
	config := DefaultConfig()
	config.Level = InfoLevel
	config.Format = FormatJSON
	config.Development = false
	config.EnableCaller = false
	config.EnableStacktrace = false
	return config
}

// LoadFromViper はViperから設定を読み込みます.
func LoadFromViper(v *viper.Viper) (*Config, error) {
	config := DefaultConfig()

	// Viperから設定を読み込み
	if v.IsSet("log") {
		if err := v.UnmarshalKey("log", config); err != nil {
			return nil, fmt.Errorf("ログ設定の読み込みエラー: %w", err)
		}
	}

	// 環境変数の優先
	if level := os.Getenv("ENVY_LOG_LEVEL"); level != "" {
		config.Level = LogLevel(strings.ToLower(level))
	}

	if format := os.Getenv("ENVY_LOG_FORMAT"); format != "" {
		config.Format = strings.ToLower(format)
	}

	if output := os.Getenv("ENVY_LOG_OUTPUT"); output != "" {
		config.Output = strings.ToLower(output)
	}

	// CLIフラグの処理
	if v.GetBool("verbose") || v.GetBool("debug") {
		config.Level = DebugLevel
	}

	if v.GetBool("quiet") {
		config.Level = ErrorLevel
	}

	// 開発モードの自動検出
	if os.Getenv("ENVY_ENV") == "development" || os.Getenv("ENVY_DEBUG") == "true" {
		config.Development = true
		if config.Level > DebugLevel {
			config.Level = DebugLevel
		}
	}

	// 検証
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate は設定の妥当性を検証します.
func (c *Config) Validate() error {
	// ログレベルの検証
	validLevels := map[LogLevel]bool{
		DebugLevel: true,
		InfoLevel:  true,
		WarnLevel:  true,
		ErrorLevel: true,
	}
	if !validLevels[c.Level] {
		return fmt.Errorf("無効なログレベル: %s", c.Level)
	}

	// フォーマットの検証
	validFormats := map[string]bool{
		FormatJSON: true,
		"console":  true,
	}
	if !validFormats[c.Format] {
		return fmt.Errorf("無効なログフォーマット: %s", c.Format)
	}

	// 出力先の検証
	validOutputs := map[string]bool{
		"stdout":   true,
		OutputFile: true,
		"syslog":   true,
	}
	if !validOutputs[c.Output] {
		return fmt.Errorf("無効な出力先: %s", c.Output)
	}

	// ファイル出力の場合のパス検証
	if c.Output == OutputFile && c.FilePath == "" {
		// デフォルトパスを設定
		c.FilePath = filepath.Join(".", "envy.log")
	}

	return nil
}

// GetLogFilePath は実際のログファイルパスを返します.
func (c *Config) GetLogFilePath() string {
	if c.FilePath == "" {
		return "envy.log"
	}

	// 日付ベースのファイル名をサポート
	if strings.Contains(c.FilePath, "%") {
		return time.Now().Format(c.FilePath)
	}

	return c.FilePath
}

// IsSensitiveKey は指定されたキーがセンシティブかどうかを判定します
func (c *Config) IsSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitive := range c.SensitiveKeys {
		if strings.Contains(lowerKey, strings.ToLower(sensitive)) {
			return true
		}
	}
	return false
}

// ConfigExample は設定ファイルの例を返します
func ConfigExample() string {
	return `# envy ログ設定例
log:
  # ログレベル: debug, info, warn, error
  level: info
  
  # ログフォーマット: json, console
  format: console
  
  # 出力先: stdout, file, syslog
  output: stdout
  
  # ファイル出力時のパス（%Y-%m-%d などの日付フォーマット対応）
  file_path: ./logs/envy-%Y-%m-%d.log
  
  # 開発モード（詳細なログ出力）
  development: false
  
  # 呼び出し元の情報を含める
  enable_caller: false
  
  # エラー時のスタックトレースを含める
  enable_stacktrace: false
  
  # ログローテーション設定
  max_size: 100      # MB
  max_backups: 3     # 保持する古いファイル数
  max_age: 28        # 日数
  compress: false    # 古いファイルを圧縮
  
  # センシティブな情報として扱うキー
  sensitive_keys:
    - password
    - secret
    - token
    - key
    - credential
    - aws_access_key_id
    - aws_secret_access_key
`
}

// MergeWithFlags はCLIフラグの設定をマージします
func (c *Config) MergeWithFlags(verbose, quiet bool) {
	if verbose {
		c.Level = DebugLevel
		c.EnableCaller = true
	}

	if quiet {
		c.Level = ErrorLevel
	}
}
