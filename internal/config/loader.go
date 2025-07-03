package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigManager manages configuration operations
type ConfigManager struct {
	configPath string
	config     *Config
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// Load loads the configuration
func (cm *ConfigManager) Load() error {
	cfg, err := Load(cm.configPath)
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	cm.config = cfg
	return nil
}

// Get returns the current configuration
func (cm *ConfigManager) Get() *Config {
	if cm.config == nil {
		cm.config = DefaultConfig()
	}
	return cm.config
}

// Create creates a new configuration file
func (cm *ConfigManager) Create(project string, interactive bool) error {
	// Check if config file already exists
	configFile := cm.configPath
	if configFile == "" {
		configFile = ".envyrc"
	}

	if _, err := os.Stat(configFile); err == nil {
		return fmt.Errorf("configuration file %s already exists", configFile)
	}

	// Create default config
	cfg := DefaultConfig()
	if project != "" {
		cfg.Project = project
	}

	// Save config
	return cfg.Save(configFile)
}

// FindConfigFile searches for .envyrc file in current directory and parents
func FindConfigFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Search in current directory and parents
	dir := cwd
	for {
		configFile := filepath.Join(dir, ".envyrc")
		if _, err := os.Stat(configFile); err == nil {
			return configFile, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

// GenerateExampleConfig generates an example configuration
func GenerateExampleConfig() string {
	return `# envy configuration file
project: myapp
default_environment: dev

aws:
  service: parameter_store  # or secrets_manager
  region: ap-northeast-1
  profile: default          # AWS profile name

environments:
  dev:
    files:
      - .env.dev
      - .env.dev.local
    path: /myapp/dev/
  
  staging:
    files:
      - .env.staging
    path: /myapp/staging/
  
  prod:
    files:
      - .env.prod
    path: /myapp/prod/
    use_secrets_manager: true  # Use Secrets Manager for production
`
}
