package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    LogLevel
		expected zapcore.Level
		wantErr  bool
	}{
		{
			name:     "Debug level",
			input:    DebugLevel,
			expected: zapcore.DebugLevel,
			wantErr:  false,
		},
		{
			name:     "Info level",
			input:    InfoLevel,
			expected: zapcore.InfoLevel,
			wantErr:  false,
		},
		{
			name:     "Warn level",
			input:    WarnLevel,
			expected: zapcore.WarnLevel,
			wantErr:  false,
		},
		{
			name:     "Error level",
			input:    ErrorLevel,
			expected: zapcore.ErrorLevel,
			wantErr:  false,
		},
		{
			name:     "String debug",
			input:    LogLevel("debug"),
			expected: zapcore.DebugLevel,
			wantErr:  false,
		},
		{
			name:     "String warning",
			input:    LogLevel("warning"),
			expected: zapcore.WarnLevel,
			wantErr:  false,
		},
		{
			name:     "Invalid level",
			input:    LogLevel("invalid"),
			expected: zapcore.InfoLevel,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := parseLogLevel(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestLoggerInitialization(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "Default config",
			config: DefaultConfig(),
		},
		{
			name:   "Development config",
			config: DevelopmentConfig(),
		},
		{
			name:   "Production config",
			config: ProductionConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.config)
			require.NoError(t, err)

			// ロガーが正しく初期化されたか確認
			assert.NotNil(t, GetLogger())
			assert.NotNil(t, GetSugaredLogger())

			// ログレベルが正しく設定されているか確認
			if tt.config.Level == DebugLevel {
				assert.True(t, IsDebugEnabled())
			}
		})
	}
}

func TestLogOutput(t *testing.T) {
	// オブザーバーを使用してログ出力をキャプチャ
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	globalLogger = logger
	globalSugar = logger.Sugar()

	// ログ出力をテスト
	Info("テストメッセージ", Field("key", "value"))
	Warn("警告メッセージ")
	Error("エラーメッセージ", ErrorField(assert.AnError))

	// 記録されたログを確認
	logs := recorded.All()
	require.Len(t, logs, 3)

	// Info ログ
	assert.Equal(t, zapcore.InfoLevel, logs[0].Level)
	assert.Equal(t, "テストメッセージ", logs[0].Message)
	assert.Equal(t, "value", logs[0].ContextMap()["key"])

	// Warn ログ
	assert.Equal(t, zapcore.WarnLevel, logs[1].Level)
	assert.Equal(t, "警告メッセージ", logs[1].Message)

	// Error ログ
	assert.Equal(t, zapcore.ErrorLevel, logs[2].Level)
	assert.Equal(t, "エラーメッセージ", logs[2].Message)
	assert.NotNil(t, logs[2].ContextMap()["error"])
}

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Short value",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "Empty value",
			input:    "",
			expected: "****",
		},
		{
			name:     "Normal value",
			input:    "password123",
			expected: "pa*******23",
		},
		{
			name:     "Long value",
			input:    "verylongsecretkey12345",
			expected: "ve******************45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitive(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSensitiveKeyDetection(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		name      string
		key       string
		sensitive bool
	}{
		{
			name:      "Password key",
			key:       "DB_PASSWORD",
			sensitive: true,
		},
		{
			name:      "Secret key",
			key:       "api_secret",
			sensitive: true,
		},
		{
			name:      "Token key",
			key:       "AUTH_TOKEN",
			sensitive: true,
		},
		{
			name:      "AWS access key",
			key:       "AWS_ACCESS_KEY_ID",
			sensitive: true,
		},
		{
			name:      "Normal key",
			key:       "DATABASE_HOST",
			sensitive: false,
		},
		{
			name:      "Port key",
			key:       "SERVER_PORT",
			sensitive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsSensitiveKey(tt.key)
			assert.Equal(t, tt.sensitive, result)
		})
	}
}

func TestJSONOutput(t *testing.T) {
	// JSON形式でログをキャプチャ
	var buf bytes.Buffer

	// カスタムエンコーダーでバッファに出力
	zapConfig := zap.NewProductionConfig()
	zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	zapConfig.Encoding = "json"
	zapConfig.OutputPaths = []string{"stdout"}

	// エンコーダーを作成してバッファに接続
	encoder := zapcore.NewJSONEncoder(zapConfig.EncoderConfig)
	writer := zapcore.AddSync(&buf)
	core := zapcore.NewCore(encoder, writer, zapcore.InfoLevel)
	logger := zap.New(core)

	globalLogger = logger
	globalSugar = logger.Sugar()

	// ログを出力
	Info("JSONログテスト", Field("number", 42), Field("flag", true))

	// JSON解析
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "JSONログテスト", logEntry["msg"])
	assert.Equal(t, float64(42), logEntry["number"])
	assert.Equal(t, true, logEntry["flag"])
}

func TestDuration(t *testing.T) {
	duration := 100 * time.Millisecond
	field := Duration("execution_time", duration)

	// フィールドが正しく作成されることを確認
	assert.Equal(t, "execution_time", field.Key)
	assert.Equal(t, zapcore.DurationType, field.Type)
}

func TestWithContext(t *testing.T) {
	// オブザーバーを使用
	core, recorded := observer.New(zapcore.InfoLevel)
	baseLogger := zap.New(core)
	globalLogger = baseLogger

	// コンテキスト付きロガーを作成
	contextLogger := WithContext(
		Field("request_id", "12345"),
		Field("user_id", "user-001"),
	)

	// コンテキスト付きロガーでログ出力
	contextLogger.Info("コンテキスト付きログ")

	// 記録されたログを確認
	logs := recorded.All()
	require.Len(t, logs, 1)

	// コンテキストフィールドが含まれているか確認
	context := logs[0].ContextMap()
	assert.Equal(t, "12345", context["request_id"])
	assert.Equal(t, "user-001", context["user_id"])
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "Invalid log level",
			config: &Config{
				Level:  LogLevel("invalid"),
				Format: "json",
				Output: "stdout",
			},
			wantErr: true,
			errMsg:  "無効なログレベル",
		},
		{
			name: "Invalid format",
			config: &Config{
				Level:  InfoLevel,
				Format: "xml",
				Output: "stdout",
			},
			wantErr: true,
			errMsg:  "無効なログフォーマット",
		},
		{
			name: "Invalid output",
			config: &Config{
				Level:  InfoLevel,
				Format: "json",
				Output: "database",
			},
			wantErr: true,
			errMsg:  "無効な出力先",
		},
		{
			name: "File output without path",
			config: &Config{
				Level:    InfoLevel,
				Format:   "json",
				Output:   "file",
				FilePath: "",
			},
			wantErr: false, // デフォルトパスが設定される
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetLevel(t *testing.T) {
	// 初期化
	config := DefaultConfig()
	config.Level = InfoLevel
	err := Init(config)
	require.NoError(t, err)

	// Info レベルではデバッグが無効
	assert.False(t, IsDebugEnabled())

	// Debug レベルに変更
	err = SetLevel(DebugLevel)
	require.NoError(t, err)

	// Debug レベルが有効になったか確認
	assert.True(t, IsDebugEnabled())

	// Error レベルに変更
	err = SetLevel(ErrorLevel)
	require.NoError(t, err)

	// Debug レベルが無効になったか確認
	assert.False(t, IsDebugEnabled())
}

func TestGetLogFilePath(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "Default path",
			config: &Config{
				FilePath: "",
			},
			expected: "envy.log",
		},
		{
			name: "Static path",
			config: &Config{
				FilePath: "/var/log/envy.log",
			},
			expected: "/var/log/envy.log",
		},
		{
			name: "Date format path",
			config: &Config{
				FilePath: "logs/envy-%Y-%m-%d.log",
			},
			expected: time.Now().Format("logs/envy-2006-01-02.log"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetLogFilePath()
			if strings.Contains(tt.config.FilePath, "%") {
				// 日付フォーマットの場合は部分一致を確認
				assert.Contains(t, result, "logs/envy-")
				assert.Contains(t, result, ".log")
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
