package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/drapon/envy/internal/config"
)

// ConfigWizard provides an interactive configuration wizard
type ConfigWizard struct {
	config        *config.Config
	awsRegions    []string
	awsServices   []string
	environments  []string
	existingFiles []string
}

// NewConfigWizard creates a new configuration wizard
func NewConfigWizard() *ConfigWizard {
	return &ConfigWizard{
		config: config.DefaultConfig(),
		awsRegions: []string{
			"us-east-1", "us-east-2", "us-west-1", "us-west-2",
			"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
			"ap-northeast-1", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2",
			"ap-south-1", "sa-east-1", "ca-central-1",
		},
		awsServices: []string{
			"parameter_store",
			"secrets_manager",
		},
		environments: []string{
			"dev", "development",
			"staging", "stage",
			"prod", "production",
			"test", "testing",
		},
	}
}

// Run runs the configuration wizard
func (w *ConfigWizard) Run() (*config.Config, error) {
	fmt.Println("Welcome to envy configuration wizard!")
	fmt.Println("This will help you set up your .envyrc file.")
	fmt.Println()

	// Project name
	w.config.Project = w.promptString("Project name", w.config.Project)

	// Default environment
	w.config.DefaultEnvironment = w.promptString("Default environment", w.config.DefaultEnvironment)

	// AWS configuration
	fmt.Println("\nAWS Configuration:")
	w.config.AWS.Service = w.promptSelect("AWS service", w.awsServices, w.config.AWS.Service)
	w.config.AWS.Region = w.promptSelect("AWS region", w.awsRegions, w.config.AWS.Region)
	w.config.AWS.Profile = w.promptString("AWS profile", w.config.AWS.Profile)

	// Environments
	fmt.Println("\nEnvironment Configuration:")
	w.configureEnvironments()

	return w.config, nil
}

// promptString prompts for a string value
func (w *ConfigWizard) promptString(label string, defaultValue string) string {
	// Use go-prompt for consistent behavior
	completer := func(d prompt.Document) []prompt.Suggest {
		return []prompt.Suggest{{Text: defaultValue}}
	}

	p := fmt.Sprintf("%s [%s]: ", label, defaultValue)
	result := prompt.Input(p, completer,
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionPreviewSuggestionTextColor(prompt.Green),
	)

	if result == "" {
		return defaultValue
	}
	return strings.TrimSpace(result)
}

// promptSelect prompts for a selection from options
func (w *ConfigWizard) promptSelect(label string, options []string, defaultValue string) string {
	completer := func(d prompt.Document) []prompt.Suggest {
		s := []prompt.Suggest{}
		for _, opt := range options {
			s = append(s, prompt.Suggest{Text: opt})
		}
		return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
	}

	p := fmt.Sprintf("%s [%s]: ", label, defaultValue)
	result := prompt.Input(p, completer,
		prompt.OptionTitle(label),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionPreviewSuggestionTextColor(prompt.Green),
		prompt.OptionSelectedSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionBGColor(prompt.DarkBlue),
	)

	if result == "" {
		return defaultValue
	}
	return result
}

// promptYesNo prompts for a yes/no answer
func (w *ConfigWizard) promptYesNo(question string, defaultYes bool) bool {
	defaultStr := "n"
	if defaultYes {
		defaultStr = "y"
	}
	
	completer := func(d prompt.Document) []prompt.Suggest {
		return []prompt.Suggest{
			{Text: "y", Description: "Yes"},
			{Text: "n", Description: "No"},
			{Text: "yes", Description: "Yes"},
			{Text: "no", Description: "No"},
		}
	}
	
	p := fmt.Sprintf("%s [%s]: ", question, defaultStr)
	result := prompt.Input(p, completer,
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionPreviewSuggestionTextColor(prompt.Green),
	)
	
	if result == "" {
		return defaultYes
	}
	
	lower := strings.ToLower(strings.TrimSpace(result))
	return lower == "y" || lower == "yes"
}

// configureEnvironments configures environment settings
func (w *ConfigWizard) configureEnvironments() {
	// Clear existing environments
	w.config.Environments = make(map[string]config.Environment)

	// If existing files found, ask whether to use them
	if len(w.existingFiles) > 0 {
		if w.promptYesNo("Use detected .env files for configuration?", true) {
			for _, file := range w.existingFiles {
				envName := extractEnvNameFromFile(file)
				env := config.Environment{
					Files: []string{file},
					Path:  fmt.Sprintf("/%s/%s/", w.config.Project, envName),
				}
				
				// Ask about Secrets Manager for production environments
				if envName == "prod" || envName == "production" {
					env.UseSecretsManager = w.promptYesNo(fmt.Sprintf("Use AWS Secrets Manager for %s?", envName), true)
				}
				
				w.config.Environments[envName] = env
			}
			
			// Set default environment if not already set
			if w.config.DefaultEnvironment == "dev" {
				// Pick first environment as default
				for envName := range w.config.Environments {
					w.config.DefaultEnvironment = envName
					break
				}
			}
			return
		}
	}

	// Add default environment
	w.addEnvironment(w.config.DefaultEnvironment)

	// Ask if user wants to add more environments
	for {
		if !w.promptYesNo("\nAdd another environment?", false) {
			break
		}

		envName := w.promptString("Environment name", "")
		if envName != "" && envName != w.config.DefaultEnvironment {
			w.addEnvironment(envName)
		}
	}
}

// addEnvironment adds a new environment configuration
func (w *ConfigWizard) addEnvironment(name string) {
	fmt.Printf("\nConfiguring environment: %s\n", name)

	env := config.Environment{
		Files: []string{},
		Path:  fmt.Sprintf("/%s/%s/", w.config.Project, name),
	}

	// Default .env file
	defaultFile := fmt.Sprintf(".env.%s", name)
	env.Files = append(env.Files, defaultFile)

	// Ask if user wants to add local override file
	if w.promptYesNo("Add local override file?", false) {
		localFile := fmt.Sprintf(".env.%s.local", name)
		env.Files = append(env.Files, localFile)
	}

	// Custom path
	customPath := w.promptString("AWS parameter path", env.Path)
	if customPath != "" {
		env.Path = customPath
	}

	// Use Secrets Manager for this environment?
	if name == "prod" || name == "production" {
		env.UseSecretsManager = w.promptYesNo("Use AWS Secrets Manager?", true)
	} else {
		env.UseSecretsManager = w.promptYesNo("Use AWS Secrets Manager?", false)
	}

	w.config.Environments[name] = env
}

// InteractiveInit runs an interactive initialization
func InteractiveInit(projectName string) error {
	// Use simple wizard for better compatibility
	return RunSimpleInteractive(projectName)
}

// InteractiveInitWithPrompt runs an interactive initialization with go-prompt
func InteractiveInitWithPrompt(projectName string) error {
	// Check if .envyrc already exists
	if _, err := os.Stat(".envyrc"); err == nil {
		fmt.Println("Error: .envyrc file already exists in current directory")
		return fmt.Errorf(".envyrc already exists")
	}

	// Detect existing .env files
	existingFiles := detectExistingEnvFiles()
	if len(existingFiles) > 0 {
		fmt.Printf("Found existing .env files: %v\n", existingFiles)
		fmt.Println()
	}

	wizard := NewConfigWizard()
	
	// Set project name if provided
	if projectName != "" {
		wizard.config.Project = projectName
	}
	
	// Set existing files for wizard
	wizard.existingFiles = existingFiles

	// Run wizard
	cfg, err := wizard.Run()
	if err != nil {
		return fmt.Errorf("wizard failed: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Save configuration
	if err := cfg.Save(".envyrc"); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("\nConfiguration saved to .envyrc")
	
	// Create example .env files
	for envName, env := range cfg.Environments {
		if len(env.Files) > 0 {
			filename := env.Files[0]
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				content := fmt.Sprintf(`# Environment variables for %s
DATABASE_URL=postgresql://localhost/myapp_%s
REDIS_URL=redis://localhost:6379
API_KEY=your-api-key-here
DEBUG=true
`, envName, envName)
				
				if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
					fmt.Printf("Warning: Failed to create %s: %v\n", filename, err)
				} else {
					fmt.Printf("Created example %s file\n", filename)
				}
			}
		}
	}

	fmt.Println("\nSetup complete! Next steps:")
	fmt.Println("1. Review and edit .envyrc if needed")
	fmt.Println("2. Edit your .env files with actual values")
	fmt.Println("3. Run 'envy push' to sync to AWS")

	return nil
}

// detectExistingEnvFiles scans for .env files
func detectExistingEnvFiles() []string {
	var envFiles []string
	
	patterns := []string{".env", ".env.*"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, match := range matches {
				// Skip example files
				if strings.HasSuffix(match, ".example") || strings.HasSuffix(match, ".sample") {
					continue
				}
				envFiles = append(envFiles, match)
			}
		}
	}
	
	return envFiles
}

// extractEnvNameFromFile extracts environment name from filename
func extractEnvNameFromFile(filename string) string {
	base := filepath.Base(filename)
	
	switch base {
	case ".env":
		return "default"
	case ".env.local":
		return "local"
	case ".env.production", ".env.prod":
		return "prod"
	case ".env.development", ".env.dev":
		return "dev" 
	case ".env.staging", ".env.stage":
		return "staging"
	case ".env.test":
		return "test"
	default:
		if len(base) > 5 && base[:5] == ".env." {
			return base[5:]
		}
		return "default"
	}
}