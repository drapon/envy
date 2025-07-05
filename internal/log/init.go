package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// InitializeLogger はenvyアプリケーション用のロガーを初期化します
func InitializeLogger(v *viper.Viper) error {
	// Viperから設定を読み込み
	config, err := LoadFromViper(v)
	if err != nil {
		return fmt.Errorf("ログ設定の読み込みエラー: %w", err)
	}

	// ロガーを初期化
	if err := Init(config); err != nil {
		return fmt.Errorf("ロガーの初期化エラー: %w", err)
	}

	// 初期化完了をログ出力
	Debug("ロガーを初期化しました",
		Field("level", string(config.Level)),
		Field("format", config.Format),
		Field("output", config.Output),
	)

	return nil
}

// SetupForCommand はコマンド実行用のロガーをセットアップします
func SetupForCommand(cmd *cobra.Command, v *viper.Viper) error {
	// Viperから設定を読み込み
	config, err := LoadFromViper(v)
	if err != nil {
		return err
	}

	// CLIフラグを確認
	verbose, _ := cmd.Flags().GetBool("verbose")
	quiet, _ := cmd.Flags().GetBool("quiet")
	config.MergeWithFlags(verbose, quiet)

	// ロガーを再初期化
	if err := Init(config); err != nil {
		return err
	}

	// コマンドコンテキストを追加
	WithFields(
		"command", cmd.Name(),
		"args", cmd.Flags().Args(),
	).Debug("コマンドを実行します")

	return nil
}

// LogCommandStart はコマンド開始時のログを出力します
func LogCommandStart(cmd *cobra.Command, args []string, startTime time.Time) {
	fields := []zap.Field{
		Field("command", cmd.Name()),
		Field("args", args),
		Field("start_time", startTime),
	}

	// 環境情報も追加
	if IsDebugEnabled() {
		fields = append(fields,
			Field("go_version", runtime.Version()),
			Field("os", runtime.GOOS),
			Field("arch", runtime.GOARCH),
		)
	}

	Info("コマンドを開始しました", fields...)
}

// LogCommandEnd はコマンド終了時のログを出力します
func LogCommandEnd(cmd *cobra.Command, startTime time.Time, err error) {
	duration := time.Since(startTime)

	fields := []zap.Field{
		Field("command", cmd.Name()),
		Duration("duration", duration),
	}

	if err != nil {
		fields = append(fields, ErrorField(err))
		Error("コマンドがエラーで終了しました", fields...)
	} else {
		Info("コマンドが正常に終了しました", fields...)
	}
}

// LogAWSOperation はAWS操作のログを出力します
func LogAWSOperation(operation string, service string, fields ...zap.Field) {
	baseFields := []zap.Field{
		Field("operation", operation),
		Field("service", service),
		Field("timestamp", time.Now()),
	}

	allFields := append(baseFields, fields...)
	Info("AWS操作を実行します", allFields...)
}

// LogAWSOperationResult はAWS操作結果のログを出力します
func LogAWSOperationResult(operation string, service string, duration time.Duration, err error, fields ...zap.Field) {
	baseFields := []zap.Field{
		Field("operation", operation),
		Field("service", service),
		Duration("duration", duration),
	}

	allFields := append(baseFields, fields...)

	if err != nil {
		allFields = append(allFields, ErrorField(err))
		Error("AWS操作がエラーで終了しました", allFields...)
	} else {
		Info("AWS操作が正常に終了しました", allFields...)
	}
}

// LogEnvSync は環境変数同期のログを出力します
func LogEnvSync(action string, source string, destination string, count int, fields ...zap.Field) {
	baseFields := []zap.Field{
		Field("action", action),
		Field("source", source),
		Field("destination", destination),
		Field("count", count),
	}

	allFields := append(baseFields, fields...)
	Info("環境変数を同期しました", allFields...)
}

// LogConfigLoad は設定ファイル読み込みのログを出力します
func LogConfigLoad(configPath string, success bool, err error) {
	fields := []zap.Field{
		Field("config_path", configPath),
		Field("success", success),
	}

	if err != nil {
		fields = append(fields, ErrorField(err))
		// 設定ファイルが見つからないのは通常の動作なのでDebugレベル
		Debug("設定ファイルの読み込みに失敗しました", fields...)
	} else {
		Debug("設定ファイルを読み込みました", fields...)
	}
}

// MaskValue はセンシティブな値をマスクしてログ用のフィールドを作成します
func MaskValue(key string, value string, config *Config) zap.Field {
	if config != nil && config.IsSensitiveKey(key) {
		return Field(key, MaskSensitive(value))
	}
	return Field(key, value)
}

// StructuredError は構造化されたエラー情報を含むログを出力します
func StructuredError(msg string, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		ErrorField(err),
		Field("error_type", fmt.Sprintf("%T", err)),
	}

	// スタックトレースが有効な場合は追加
	if IsDebugEnabled() {
		allFields = append(allFields, zap.Stack("stacktrace"))
	}

	allFields = append(allFields, fields...)
	Error(msg, allFields...)
}

// FlushLogs はバッファされたログをフラッシュします（defer で使用）
func FlushLogs() {
	if err := Sync(); err != nil {
		// Sync自体のエラーは標準エラー出力に出力
		// ただし、stdout/stderrの場合はよくあるエラーなので、デバッグモードのみ表示
		errStr := err.Error()
		if IsDebugEnabled() &&
			!strings.Contains(errStr, "inappropriate ioctl") &&
			!strings.Contains(errStr, "bad file descriptor") &&
			!strings.Contains(errStr, "invalid argument") {
			fmt.Fprintf(os.Stderr, "ログのフラッシュに失敗しました: %v\n", err)
		}
	}
}

// InitLogger is a simple initialization function for testing
func InitLogger(debug bool, level string) {
	cfg := DefaultConfig()
	if debug {
		cfg.Development = true
	}
	if level != "" {
		switch strings.ToLower(level) {
		case "debug":
			cfg.Level = DebugLevel
		case "info":
			cfg.Level = InfoLevel
		case "warn":
			cfg.Level = WarnLevel
		case "error":
			cfg.Level = ErrorLevel
		}
	}
	Init(cfg)
}
