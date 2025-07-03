# envy Logging System

Implementation of structured logging system for the envy project. Provides high-performance and flexible logging functionality using the zap library.

## Features

- **Structured Logging**: Output in JSON or console format
- **Log Levels**: Four levels - Debug, Info, Warn, Error
- **International Support**: Error messages and log output in English
- **Sensitive Information Masking**: Automatic masking of passwords and tokens
- **Performance Measurement**: Automatic execution time recording
- **Flexible Configuration**: Control via configuration files, environment variables, and CLI flags

## Usage

### Initialization

```go
import (
    "github.com/drapon/envy/internal/log"
    "github.com/spf13/viper"
)

// Initialize by reading configuration from Viper
if err := log.InitializeLogger(viper.GetViper()); err != nil {
    // Error handling
}

// Flush logs when program exits
defer log.FlushLogs()
```

### Basic Log Output

```go
// Simple logs
log.Info("Process started")
log.Warn("Warning: Retrying")
log.Error("An error occurred")

// Logs with fields
log.Info("User logged in",
    log.Field("user_id", userID),
    log.Field("ip_address", ipAddr),
)

// Formatted logs
log.Infof("Process completed: %d items", count)
log.Errorf("File not found: %s", filename)
```

### Contextual Logging

```go
// Create logger with specific context
logger := log.WithFields(
    "service", "aws",
    "region", "us-east-1",
)

// All output from this logger includes the above fields
logger.Info("Accessing S3 bucket")
logger.Error("Connection error", log.ErrorField(err))
```

### Command Execution Logging

```go
// At command start
log.LogCommandStart(cmd, args, time.Now())

// At command end
log.LogCommandEnd(cmd, startTime, err)

// AWS operation logging
log.LogAWSOperation("GetParameter", "ssm",
    log.Field("parameter", paramName),
)

// Environment variable sync logging
log.LogEnvSync("push", "local", "parameter_store", count,
    log.Field("environment", envName),
)
```

### Handling Sensitive Information

```go
// Automatic masking
config := log.DefaultConfig()
maskedField := log.MaskValue("api_key", actualValue, config)

// Manual masking
log.Info("API key configured",
    log.Field("api_key", log.MaskSensitive(apiKey)),
)
```

## Configuration

### Configuration File (.envyrc)

```yaml
log:
  # Log level: debug, info, warn, error
  level: info
  
  # Format: json, console
  format: console
  
  # Output destination: stdout, file, syslog
  output: stdout
  
  # File path for file output
  file_path: ./logs/envy.log
  
  # Development mode (detailed logs)
  development: false
  
  # Sensitive keys
  sensitive_keys:
    - password
    - secret
    - token
    - api_key
```

### Environment Variables

```bash
# Configure log level
export ENVY_LOG_LEVEL=debug

# Configure log format
export ENVY_LOG_FORMAT=json

# Configure output destination
export ENVY_LOG_OUTPUT=file

# Enable development mode
export ENVY_ENV=development
# or
export ENVY_DEBUG=true
```

### CLI Flags

```bash
# Enable verbose logging
envy push --verbose

# Output errors only
envy pull --quiet

# Debug mode
envy list --debug
```

## Log Level Guidelines

### Debug
- Detailed debugging information
- Variable values, function input/output
- Development troubleshooting

```go
log.Debug("Loading configuration", log.Field("path", configPath))
```

### Info
- General operation information
- Process start/completion
- Important status changes

```go
log.Info("Environment variables pushed", log.Field("count", 10))
```

### Warn
- Warnings (processing can continue)
- Retries and alternative processing
- Use of deprecated features

```go
log.Warn("Connection timed out. Retrying", log.Field("attempt", 2))
```

### Error
- Errors (processing failed)
- Exceptional situations
- User intervention required

```go
log.Error("Failed to connect to AWS", log.ErrorField(err))
```

## Preset Configurations

### Development Environment

```go
config := log.DevelopmentConfig()
// - Level: Debug
// - Caller info: Enabled
// - Stack trace: Enabled
// - Color output: Enabled
```

### Production Environment

```go
config := log.ProductionConfig()
// - Level: Info
// - Format: JSON
// - Caller info: Disabled
// - Stack trace: Disabled
```

## Performance

- High-speed structured logging with zap
- Zero-allocation field creation
- Asynchronous log output (buffering)
- Conditional log evaluation

```go
// Not evaluated if debug level is disabled
if log.IsDebugEnabled() {
    log.Debug("Result of expensive operation", log.Field("data", expensiveOperation()))
}
```

## Troubleshooting

### No Log Output

1. Check log level settings
2. Verify configuration file loading
3. Check environment variable priority

### Performance Issues

1. Set appropriate log level
2. Check rotation settings for file output
3. Remove unnecessary debug logs

### Character Encoding Issues

1. Check terminal encoding
2. Try JSON format output
3. Use file output