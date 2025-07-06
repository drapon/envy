package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// グローバルロガー
	globalLogger *zap.Logger
	// グローバルSugaredLogger（より使いやすいAPI）
	globalSugar *zap.SugaredLogger
)

// LogLevel はログレベルを表す型
type LogLevel string

const (
	// DebugLevel 詳細なデバッグ情報
	DebugLevel LogLevel = "debug"
	// InfoLevel 一般的な操作情報
	InfoLevel LogLevel = "info"
	// WarnLevel 警告（処理は続行）
	WarnLevel LogLevel = "warn"
	// ErrorLevel エラー（処理停止）
	ErrorLevel LogLevel = "error"
)

// Init はグローバルロガーを初期化します
func Init(config *Config) error {
	cfg := zap.NewProductionConfig()

	// ログレベルの設定
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return fmt.Errorf("ログレベルの解析エラー: %w", err)
	}
	cfg.Level = zap.NewAtomicLevelAt(level)

	// エンコーディングの設定
	cfg.Encoding = config.Format

	// タイムスタンプフォーマット
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 日本語対応のメッセージキー
	cfg.EncoderConfig.MessageKey = "message"

	// 開発モードの設定
	if config.Development {
		cfg = zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(level)
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		// コンソール出力の場合、より読みやすいフォーマットに
		if config.Format == "console" {
			cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
				enc.AppendString(t.Format("15:04:05.000"))
			}
		}
	}

	// 出力先の設定
	switch config.Output {
	case "stdout":
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	case "file":
		if config.FilePath == "" {
			return fmt.Errorf("ファイル出力が指定されていますが、ファイルパスが空です")
		}
		cfg.OutputPaths = []string{config.FilePath}
		cfg.ErrorOutputPaths = []string{config.FilePath}
	case "syslog":
		// syslogはzapのプラグインで対応可能
		// ここでは簡単のためstdoutにフォールバック
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	default:
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	}

	// ロガーの構築
	logger, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("ロガーの構築エラー: %w", err)
	}

	// グローバルロガーの設定
	globalLogger = logger
	globalSugar = logger.Sugar()

	// zapのグローバルロガーも置き換え
	zap.ReplaceGlobals(logger)

	return nil
}

// GetLogger はグローバルロガーを返します
func GetLogger() *zap.Logger {
	if globalLogger == nil {
		// 初期化されていない場合はデフォルトロガーを作成
		logger, _ := zap.NewProduction()
		globalLogger = logger
		globalSugar = logger.Sugar()
	}
	return globalLogger
}

// GetSugaredLogger はSugaredLoggerを返します
func GetSugaredLogger() *zap.SugaredLogger {
	if globalSugar == nil {
		GetLogger() // 初期化を強制
	}
	return globalSugar
}

// WithContext はコンテキスト情報を追加したロガーを返します
func WithContext(fields ...zap.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

// WithFields はフィールドを追加したSugaredLoggerを返します
func WithFields(keysAndValues ...interface{}) *zap.SugaredLogger {
	return GetSugaredLogger().With(keysAndValues...)
}

// Debug はデバッグレベルのログを出力します
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Debugf はフォーマット済みデバッグレベルのログを出力します
func Debugf(template string, args ...interface{}) {
	GetSugaredLogger().Debugf(template, args...)
}

// Info は情報レベルのログを出力します
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Infof はフォーマット済み情報レベルのログを出力します
func Infof(template string, args ...interface{}) {
	GetSugaredLogger().Infof(template, args...)
}

// Warn は警告レベルのログを出力します
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Warnf はフォーマット済み警告レベルのログを出力します
func Warnf(template string, args ...interface{}) {
	GetSugaredLogger().Warnf(template, args...)
}

// Error はエラーレベルのログを出力します
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Errorf はフォーマット済みエラーレベルのログを出力します
func Errorf(template string, args ...interface{}) {
	GetSugaredLogger().Errorf(template, args...)
}

// Fatal は致命的エラーのログを出力し、プログラムを終了します
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// Fatalf はフォーマット済み致命的エラーのログを出力し、プログラムを終了します
func Fatalf(template string, args ...interface{}) {
	GetSugaredLogger().Fatalf(template, args...)
}

// Sync はバッファされたログをフラッシュします
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// InitTestLogger initializes a test logger and returns it
func InitTestLogger() *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	cfg.DisableStacktrace = true
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}

	logger, _ := cfg.Build()
	globalLogger = logger
	globalSugar = logger.Sugar()

	return logger
}

// MaskSensitive はセンシティブな情報をマスクします
func MaskSensitive(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	// 最初と最後の2文字を残してマスク
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

// Field はzap.Fieldのエイリアス
func Field(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

// Duration は実行時間を記録するためのフィールドを作成します
func Duration(key string, duration time.Duration) zap.Field {
	return zap.Duration(key, duration)
}

// ErrorField はエラーフィールドを作成します
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

// parseLogLevel は文字列のログレベルをzapcore.Levelに変換します
func parseLogLevel(level LogLevel) (zapcore.Level, error) {
	switch level {
	case DebugLevel:
		return zapcore.DebugLevel, nil
	case InfoLevel:
		return zapcore.InfoLevel, nil
	case WarnLevel:
		return zapcore.WarnLevel, nil
	case ErrorLevel:
		return zapcore.ErrorLevel, nil
	default:
		// 環境変数から直接文字列が来る場合の対応
		switch strings.ToLower(string(level)) {
		case "debug":
			return zapcore.DebugLevel, nil
		case "info":
			return zapcore.InfoLevel, nil
		case "warn", "warning":
			return zapcore.WarnLevel, nil
		case "error":
			return zapcore.ErrorLevel, nil
		default:
			return zapcore.InfoLevel, fmt.Errorf("不明なログレベル: %s", level)
		}
	}
}

// SetLevel は動的にログレベルを変更します
func SetLevel(level LogLevel) error {
	zapLevel, err := parseLogLevel(level)
	if err != nil {
		return err
	}

	// 現在のロガーの設定を取得して、レベルだけ変更
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	// 新しいロガーを作成
	logger, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("ロガーの再構築エラー: %w", err)
	}

	// グローバルロガーを更新
	globalLogger = logger
	globalSugar = logger.Sugar()
	zap.ReplaceGlobals(logger)

	return nil
}

// IsDebugEnabled はデバッグレベルが有効かどうかを返します
func IsDebugEnabled() bool {
	return GetLogger().Core().Enabled(zapcore.DebugLevel)
}

// GetLevelFromEnv は環境変数からログレベルを取得します
func GetLevelFromEnv() LogLevel {
	level := os.Getenv("ENVY_LOG_LEVEL")
	if level == "" {
		return InfoLevel
	}
	return LogLevel(strings.ToLower(level))
}
