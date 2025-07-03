# Error Handling Package

## Overview

The `errors` package provides a unified error handling system used throughout the envy project. It includes features such as custom error types, error messages, retry mechanisms, timeout handling, and more.

## Main Features

### 1. Custom Error Type (EnvyError)

```go
// Create error
err := errors.New(errors.ErrConfigNotFound, "Configuration file not found")
    .WithDetails("file", ".envyrc")
    .WithRetriable(false)

// Check error type
if errors.IsConfigError(err) {
    // Handle configuration-related errors
}
```

### 2. Error Categories

- **Configuration Errors** (`ErrConfig*`): Configuration file related
- **Validation Errors** (`ErrValidation*`): Input validation related
- **AWS Errors** (`ErrAWS*`): AWS API related
- **File Errors** (`ErrFile*`): File operation related
- **Network Errors** (`ErrNetwork*`): Network related

### 3. User Messages

```go
// User-friendly messages
fmt.Println(err.UserMessage())
// Output: "Configuration file not found. Please run 'envy init' command to initialize."
```

### 4. Error Formatters

```go
// Standard error output
errors.PrintError(err)

// Verbose error output
errors.PrintErrorVerbose(err)

// Error with context
errors.FormatWithContext(err, errors.ErrorContext{
    Operation:   "AWS Sync",
    Environment: "production",
    Region:      "ap-northeast-1",
})
```

### 5. Error Aggregation

```go
aggregator := errors.NewAggregator()

for _, file := range files {
    if err := processFile(file); err != nil {
        aggregator.Add(err)
    }
}

if aggregator.HasErrors() {
    return aggregator.Error()
}
```

## Retry Mechanism

### Basic Usage

```go
// Retry with default settings
err := retry.WithRetry(ctx, func() error {
    return someOperation()
})

// Retry with AWS settings
err := retry.WithAWSRetry(ctx, func() error {
    return awsClient.GetParameter(name)
})

// Retry with custom settings
retryer := retry.New(retry.Config{
    MaxAttempts:  5,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Strategy:     retry.StrategyExponential,
})

err := retryer.Do(ctx, func(ctx context.Context) error {
    return someOperation()
})
```

### Retry Notifications

```go
err := retryer.DoWithNotify(ctx, operation, func(err error, attempt int, delay time.Duration) {
    log.Printf("Retry %d/%d: %v (next: %s)", attempt, maxAttempts, err, delay)
})
```

## Timeout Handling

```go
// Operation-specific timeout
err := retry.WithAWSTimeout(ctx, func(ctx context.Context) error {
    return awsOperation(ctx)
})

err := retry.WithNetworkTimeout(ctx, func(ctx context.Context) error {
    return httpClient.Do(req)
})

// Custom timeout
err := retry.WithTimeout(ctx, 30*time.Second, func(ctx context.Context) error {
    return longRunningOperation(ctx)
})
```

## Usage Examples

### Command Usage

```go
func (c *PullCommand) Execute(ctx context.Context) error {
    // Input validation
    if c.environment == "" {
        return errors.New(errors.ErrRequiredField, "Environment name is required")
            .WithDetails("field", "environment")
    }

    // AWS operation (with retry)
    params, err := retry.WithAWSRetry(ctx, func() error {
        return c.awsClient.GetParameters(c.environment)
    })
    
    if err != nil {
        // Enhance and return error
        return errors.EnhanceAWSError(err, "GetParameters", c.environment)
    }

    // Success message
    errors.PrintSuccess(fmt.Sprintf("Retrieved %d parameters", len(params)))
    return nil
}
```

### Error Handling Best Practices

1. **Early Return**: Return immediately when an error occurs
2. **Add Context**: Add operation and resource information to errors
3. **Appropriate Error Types**: Use appropriate error codes for the situation
4. **Retriable**: Set network and rate limit errors as retriable
5. **User-Friendly**: Provide clear and understandable messages

## Testing

```go
// Error testing
func TestOperation(t *testing.T) {
    err := someOperation()
    
    // Check error type
    assert.True(t, errors.IsAWSError(err))
    
    // Check error code
    assert.Equal(t, errors.ErrParameterNotFound, errors.GetErrorCode(err))
    
    // Check retriability
    assert.True(t, errors.IsRetriable(err))
}
```