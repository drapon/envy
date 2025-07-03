# Cache System

The envy project's cache system improves performance by caching data retrieved from AWS Parameter Store/Secrets Manager, configuration file parsing results, .env file parsing results, validation rule results, and more.

## Main Features

### Cache Types
- **Memory Cache**: In-memory cache (fast, lost on process termination)
- **Disk Cache**: Disk-based cache (persistent, shared between processes)
- **Hybrid Cache**: Memory + Disk cache (benefits of both)

### Key Features
- **TTL (Time To Live)**: Cache entry expiration
- **LRU Eviction Strategy**: Eviction strategy when memory usage limits are reached
- **Encrypted Cache**: Encrypted storage of sensitive information
- **Auto-invalidation**: File update detection
- **Statistics**: Information on hit rate, entry count, size, etc.

## Configuration

### Configuration in .envyrc file

```yaml
# Basic settings
project: myapp
default_environment: dev

# Cache settings
cache:
  enabled: true                    # Enable/disable cache
  type: hybrid                     # memory, disk, hybrid
  ttl: 1h                         # Default TTL
  max_size: 100MB                 # Maximum cache size
  max_entries: 1000               # Maximum number of entries
  dir: ~/.envy/cache              # Cache directory
  encryption_key_file: ~/.envy/key # Encryption key file

# AWS settings
aws:
  service: parameter_store
  region: us-east-1
  profile: default

# Environment settings
environments:
  dev:
    files: [".env.dev"]
    path: "/myapp/dev/"
```

### Configuration via Environment Variables

```bash
# Cache settings
export ENVY_CACHE_ENABLED=true
export ENVY_CACHE_TYPE=hybrid
export ENVY_CACHE_TTL=1h
export ENVY_CACHE_MAX_SIZE=100MB
export ENVY_CACHE_DIR=~/.envy/cache
```

## Command Line Control

### Global Flags

```bash
# Run with cache disabled
envy pull --no-cache

# Clear cache before running
envy push --clear-cache

# Both at the same time
envy list --no-cache --clear-cache
```

### Cache-specific Commands

```bash
# Display cache statistics
envy cache --stats

# Clear all cache
envy cache --clear

# Default (display statistics)
envy cache
```

## Programmatic Usage

### Basic Usage Example

```go
package main

import (
    "time"
    "github.com/drapon/envy/internal/cache"
)

func main() {
    // Cache configuration
    config := &cache.CacheConfig{
        Type:       cache.HybridCache,
        TTL:        1 * time.Hour,
        MaxSize:    100 * 1024 * 1024, // 100MB
        MaxEntries: 1000,
        CacheDir:   "~/.envy/cache",
        Enabled:    true,
    }

    // Create cache manager
    manager, err := cache.NewCacheManager(config)
    if err != nil {
        panic(err)
    }
    defer manager.Close()

    // GetOrSet pattern: Get from cache, or generate if not found
    result, err := manager.GetOrSet(
        "my_key",
        30*time.Minute,
        func() (interface{}, error) {
            // Expensive operation (AWS API call, etc.)
            return "expensive_result", nil
        },
    )
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Result:", result)
}
```

### Cache Key Generation

```go
// File-based cache key
key, modTime, err := cache.FileBasedCacheKey("config", ".envyrc")

// Custom key building
key := cache.NewCacheKeyBuilder("aws_env").
    Add("production").
    Add("us-east-1").
    AddF("path_%s", "/myapp/prod/").
    Build()
```

### Sensitive Data Encryption

```go
// Specify sensitive data via metadata
err := cacheManager.cache.SetWithMetadata(
    "password_key",
    "secret_password",
    1*time.Hour,
    map[string]interface{}{
        "sensitive": true,
    },
)
```

### Statistics Retrieval

```go
stats := cacheManager.Stats()
fmt.Printf("Hit Rate: %.2f%%\n", stats.HitRate()*100)
fmt.Printf("Entry Count: %d\n", stats.Entries)
fmt.Printf("Cache Size: %s\n", cache.FormatSize(stats.Size))
```

## Security

### File Permissions
- Cache files are stored with 600 permissions
- Cache directory is created with 700 permissions

### Encryption
- Sensitive data is encrypted with AES-GCM
- Encryption key is retrieved from file or environment variable
- Automatic encryption when key contains sensitive patterns (password, secret, token, etc.)

### Key Generation
- Cache file names are generated with SHA-256 hash
- Original key information is difficult to guess

## Performance

### Memory Usage
- Entry count limit: `max_entries`
- Size limit: `max_size`
- LRU eviction strategy controls memory usage

### Disk I/O
- Atomic file writing (temporary file → rename)
- Directory hierarchy for file system load balancing
- Periodic cleanup of expired files

### AWS API Call Reduction
- Cache AWS Parameter Store/Secrets Manager calls
- Default 15-minute TTL reduces AWS API costs
- Automatic invalidation on file changes

## Troubleshooting

### Debugging

```bash
# Enable debug logs and check cache statistics
envy cache --stats --debug

# Disable cache to isolate issues
envy pull --no-cache --debug
```

### Common Issues

1. **Cache directory access permission error**
   ```bash
   chmod 700 ~/.envy/cache
   ```

2. **Encryption key issues**
   ```bash
   # Check encryption key file permissions
   ls -la ~/.envy/key
   chmod 600 ~/.envy/key
   ```

3. **Disk space shortage**
   ```bash
   # Clear cache
   envy cache --clear
   ```

4. **Accumulation of expired files**
   ```bash
   # Manual cleanup
   envy cache --clear
   ```

## Design Philosophy

The envy cache system is designed based on the following principles:

1. **Transparency**: Returns the same result regardless of cache presence
2. **Security**: Proper encryption and permission management of sensitive data
3. **Performance**: Reduced AWS API calls and fast local access
4. **Reliability**: Automatic file update detection and cache invalidation
5. **Operability**: Rich statistics and debugging features

This allows developers to achieve fast and secure environment variable management.