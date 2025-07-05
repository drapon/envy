package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the envy configuration
type Config struct {
	Project            string                 `mapstructure:"project"`
	DefaultEnvironment string                 `mapstructure:"default_environment"`
	AWS                AWSConfig              `mapstructure:"aws"`
	Cache              CacheConfig            `mapstructure:"cache"`
	Memory             MemoryConfig           `mapstructure:"memory"`
	Performance        PerformanceConfig      `mapstructure:"performance"`
	Environments       map[string]Environment `mapstructure:"environments"`
}

// AWSConfig represents AWS-specific configuration
type AWSConfig struct {
	Service string `mapstructure:"service"` // parameter_store or secrets_manager
	Region  string `mapstructure:"region"`
	Profile string `mapstructure:"profile"`
}

// CacheConfig represents cache-specific configuration
type CacheConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	Type              string `mapstructure:"type"`                // memory, disk, hybrid
	TTL               string `mapstructure:"ttl"`                 // duration string like "1h", "30m"
	MaxSize           string `mapstructure:"max_size"`            // size string like "100MB", "1GB"
	MaxEntries        int    `mapstructure:"max_entries"`         // maximum number of entries
	Dir               string `mapstructure:"dir"`                 // cache directory
	EncryptionKey     string `mapstructure:"encryption_key"`      // encryption key for sensitive data
	EncryptionKeyFile string `mapstructure:"encryption_key_file"` // file containing encryption key
}

// MemoryConfig represents memory optimization configuration
type MemoryConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	PoolEnabled       bool          `mapstructure:"pool_enabled"`
	MonitoringEnabled bool          `mapstructure:"monitoring_enabled"`
	StringPoolSize    int64         `mapstructure:"string_pool_size"`
	BytePoolSize      int64         `mapstructure:"byte_pool_size"`
	MapPoolSize       int64         `mapstructure:"map_pool_size"`
	GCInterval        time.Duration `mapstructure:"gc_interval"`
	MemoryThreshold   int64         `mapstructure:"memory_threshold"`
}

// PerformanceConfig represents performance optimization configuration
type PerformanceConfig struct {
	BatchSize        int  `mapstructure:"batch_size"`
	WorkerCount      int  `mapstructure:"worker_count"`
	StreamingEnabled bool `mapstructure:"streaming_enabled"`
	BufferSize       int  `mapstructure:"buffer_size"`
	MaxLineSize      int  `mapstructure:"max_line_size"`
}

// Environment represents an environment configuration
type Environment struct {
	Files             []string `mapstructure:"files"`
	Path              string   `mapstructure:"path"`
	UseSecretsManager bool     `mapstructure:"use_secrets_manager"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	// Use fixed default project name for consistency
	projectName := "myapp"

	return &Config{
		Project:            projectName,
		DefaultEnvironment: "dev",
		AWS: AWSConfig{
			Service: "parameter_store",
			Region:  "us-east-1",
			Profile: "default",
		},
		Cache: CacheConfig{
			Enabled:    true,
			Type:       "hybrid",
			TTL:        "1h",
			MaxSize:    "100MB",
			MaxEntries: 1000,
		},
		Memory: MemoryConfig{
			Enabled:           true,
			PoolEnabled:       true,
			MonitoringEnabled: true,
			StringPoolSize:    1024,
			BytePoolSize:      64 * 1024, // 64KB
			MapPoolSize:       100,
			GCInterval:        30 * time.Second,
			MemoryThreshold:   100 * 1024 * 1024, // 100MB
		},
		Performance: PerformanceConfig{
			BatchSize:        50,
			WorkerCount:      4,
			StreamingEnabled: true,
			BufferSize:       8192,
			MaxLineSize:      64 * 1024, // 64KB
		},
		Environments: map[string]Environment{
			"dev": {
				Files: []string{".env.dev"},
				Path:  fmt.Sprintf("/%s/dev/", projectName),
			},
		},
	}
}

// Load loads the configuration from file
func Load(configFile string) (*Config, error) {
	v := viper.New()
	cfg := &Config{}

	if configFile != "" {
		v.SetConfigFile(configFile)
		v.SetConfigType("yaml")
	} else {
		// Search for .envyrc in current directory and parents
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}

		v.SetConfigName(".envyrc")
		v.SetConfigType("yaml")

		// Add current directory and all parent directories to search path
		dir := cwd
		for {
			v.AddConfigPath(dir)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Set environment variable prefix
	v.SetEnvPrefix("ENVY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind environment variables
	v.BindEnv("project")
	v.BindEnv("default_environment")
	v.BindEnv("aws.region")
	v.BindEnv("aws.profile")
	v.BindEnv("aws.service")

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; use defaults
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Fix environments with dots in their names
	// Viper interprets dots in YAML keys as nested structures, so "production.local"
	// becomes nested as environments.production.local instead of environments["production.local"]
	// We need to manually reconstruct the map to handle both regular and dotted environment names
	if envMap := v.GetStringMap("environments"); envMap != nil {
		cfg.Environments = make(map[string]Environment)
		for key, value := range envMap {
			if envConfig, ok := value.(map[string]interface{}); ok {
				env := Environment{}

				// Check if this is a properly formed environment config
				if files, hasFiles := envConfig["files"]; hasFiles {
					// This is a complete environment configuration
					if fileList, ok := files.([]interface{}); ok {
						env.Files = make([]string, 0, len(fileList))
						for _, f := range fileList {
							if str, ok := f.(string); ok {
								env.Files = append(env.Files, str)
							}
						}
					}

					if path, ok := envConfig["path"].(string); ok {
						env.Path = path
					}

					if useSecretsManager, ok := envConfig["use_secrets_manager"].(bool); ok {
						env.UseSecretsManager = useSecretsManager
					}

					cfg.Environments[key] = env
				} else {
					// This might be a nested structure due to dots in the name
					// We need to check for nested environments
					for nestedKey, nestedValue := range envConfig {
						if nestedEnvConfig, ok := nestedValue.(map[string]interface{}); ok {
							if _, hasFiles := nestedEnvConfig["files"]; hasFiles {
								// This is an environment with a dotted name
								fullKey := key + "." + nestedKey
								env := Environment{}

								if files, ok := nestedEnvConfig["files"].([]interface{}); ok {
									env.Files = make([]string, 0, len(files))
									for _, f := range files {
										if str, ok := f.(string); ok {
											env.Files = append(env.Files, str)
										}
									}
								}

								if path, ok := nestedEnvConfig["path"].(string); ok {
									env.Path = path
								}

								if useSecretsManager, ok := nestedEnvConfig["use_secrets_manager"].(bool); ok {
									env.UseSecretsManager = useSecretsManager
								}

								cfg.Environments[fullKey] = env
							}
						}
					}
				}
			}
		}
	}

	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save(filename string) error {
	if filename == "" {
		filename = ".envyrc"
	}

	v := viper.New()
	v.SetConfigType("yaml")

	v.Set("project", c.Project)
	v.Set("default_environment", c.DefaultEnvironment)
	v.Set("aws", c.AWS)
	v.Set("cache", c.Cache)
	v.Set("memory", c.Memory)
	v.Set("performance", c.Performance)
	v.Set("environments", c.Environments)

	// WriteConfigAs requires the file extension to determine the type
	// If the filename doesn't have .yaml or .yml extension, we need to handle it
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		// Create a temporary yaml file and then rename it
		tempFile := filename + ".yaml"
		if err := v.WriteConfigAs(tempFile); err != nil {
			return err
		}
		return os.Rename(tempFile, filename)
	}

	return v.WriteConfigAs(filename)
}

// GetEnvironment returns the configuration for a specific environment
func (c *Config) GetEnvironment(name string) (*Environment, error) {
	if name == "" {
		name = c.DefaultEnvironment
	}

	env, ok := c.Environments[name]
	if !ok {
		return nil, fmt.Errorf("environment '%s' not found in configuration", name)
	}

	return &env, nil
}

// GetAWSService returns the AWS service to use for the given environment
func (c *Config) GetAWSService(envName string) string {
	env, err := c.GetEnvironment(envName)
	if err != nil {
		return c.AWS.Service
	}

	if env.UseSecretsManager {
		return "secrets_manager"
	}

	return c.AWS.Service
}

// GetParameterPath returns the AWS parameter path for the given environment
func (c *Config) GetParameterPath(envName string) string {
	env, err := c.GetEnvironment(envName)
	if err != nil {
		return fmt.Sprintf("/%s/%s/", c.Project, envName)
	}

	if env.Path != "" {
		return env.Path
	}

	return fmt.Sprintf("/%s/%s/", c.Project, envName)
}

// GetMemoryConfig returns the memory configuration
func (c *Config) GetMemoryConfig() MemoryConfig {
	return c.Memory
}

// GetPerformanceConfig returns the performance configuration
func (c *Config) GetPerformanceConfig() PerformanceConfig {
	return c.Performance
}

// IsMemoryOptimizationEnabled returns whether memory optimization is enabled
func (c *Config) IsMemoryOptimizationEnabled() bool {
	return c.Memory.Enabled
}

// IsStreamingEnabled returns whether streaming is enabled
func (c *Config) IsStreamingEnabled() bool {
	return c.Performance.StreamingEnabled
}

// GetBatchSize returns the configured batch size
func (c *Config) GetBatchSize() int {
	if c.Performance.BatchSize <= 0 {
		return 50 // Default batch size
	}
	return c.Performance.BatchSize
}

// GetWorkerCount returns the configured worker count
func (c *Config) GetWorkerCount() int {
	if c.Performance.WorkerCount <= 0 {
		return 4 // Default worker count
	}
	return c.Performance.WorkerCount
}

// GetBufferSize returns the configured buffer size
func (c *Config) GetBufferSize() int {
	if c.Performance.BufferSize <= 0 {
		return 8192 // Default buffer size
	}
	return c.Performance.BufferSize
}

// GetMemoryThreshold returns the memory threshold in bytes
func (c *Config) GetMemoryThreshold() int64 {
	if c.Memory.MemoryThreshold <= 0 {
		return 100 * 1024 * 1024 // Default 100MB
	}
	return c.Memory.MemoryThreshold
}

// GetCacheConfig returns the cache configuration
func (c *Config) GetCacheConfig() CacheConfig {
	return c.Cache
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Project == "" {
		return fmt.Errorf("project name is required")
	}

	if c.DefaultEnvironment == "" {
		return fmt.Errorf("default_environment is required")
	}

	if c.AWS.Region == "" {
		return fmt.Errorf("aws.region is required")
	}

	if c.AWS.Service != "parameter_store" && c.AWS.Service != "secrets_manager" {
		return fmt.Errorf("aws.service must be either 'parameter_store' or 'secrets_manager'")
	}

	if len(c.Environments) == 0 {
		return fmt.Errorf("at least one environment must be defined")
	}

	for name, env := range c.Environments {
		if len(env.Files) == 0 {
			return fmt.Errorf("environment '%s' must have at least one file", name)
		}
		if env.Path == "" {
			return fmt.Errorf("environment '%s' must have a path", name)
		}
	}

	// Validate memory configuration
	if c.Memory.Enabled {
		if c.Memory.StringPoolSize < 0 {
			return fmt.Errorf("memory.string_pool_size must be non-negative")
		}
		if c.Memory.BytePoolSize < 0 {
			return fmt.Errorf("memory.byte_pool_size must be non-negative")
		}
		if c.Memory.MapPoolSize < 0 {
			return fmt.Errorf("memory.map_pool_size must be non-negative")
		}
		if c.Memory.MemoryThreshold < 0 {
			return fmt.Errorf("memory.memory_threshold must be non-negative")
		}
	}

	// Validate performance configuration
	if c.Performance.BatchSize < 0 {
		return fmt.Errorf("performance.batch_size must be non-negative")
	}
	if c.Performance.WorkerCount < 0 {
		return fmt.Errorf("performance.worker_count must be non-negative")
	}
	if c.Performance.BufferSize < 0 {
		return fmt.Errorf("performance.buffer_size must be non-negative")
	}
	if c.Performance.MaxLineSize < 0 {
		return fmt.Errorf("performance.max_line_size must be non-negative")
	}

	return nil
}
