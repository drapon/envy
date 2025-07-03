package wizard

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/drapon/envy/internal/config"
)

// SimpleWizard provides a simple interactive configuration wizard
type SimpleWizard struct {
	config        *config.Config
	scanner       *bufio.Scanner
	existingFiles []string
}

// NewSimpleWizard creates a new simple wizard
func NewSimpleWizard() *SimpleWizard {
	return &SimpleWizard{
		config:  config.DefaultConfig(),
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// RunSimpleInteractive runs a simplified interactive initialization
func RunSimpleInteractive(projectName string) error {
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

	wizard := NewSimpleWizard()
	wizard.existingFiles = existingFiles
	
	// Set project name if provided
	if projectName != "" {
		wizard.config.Project = projectName
	}

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
	
	// Create example .env files only if no existing files
	if len(existingFiles) == 0 {
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
	}

	fmt.Println("\nSetup complete! Next steps:")
	fmt.Println("1. Review and edit .envyrc if needed")
	fmt.Println("2. Edit your .env files with actual values")
	fmt.Println("3. Run 'envy push' to sync to AWS")

	return nil
}

// Run runs the simple wizard
func (w *SimpleWizard) Run() (*config.Config, error) {
	fmt.Println("Welcome to envy configuration wizard!")
	fmt.Println("This will help you set up your .envyrc file.")
	fmt.Println()

	// Project name
	w.config.Project = w.promptString("Project name", w.config.Project)

	// AWS configuration
	fmt.Println("\nAWS Configuration:")
	w.config.AWS.Service = w.promptChoice("AWS service", []string{"parameter_store", "secrets_manager"}, w.config.AWS.Service)
	w.config.AWS.Region = w.promptString("AWS region", w.config.AWS.Region)
	w.config.AWS.Profile = w.promptString("AWS profile", w.config.AWS.Profile)

	// Environments
	fmt.Println("\nEnvironment Configuration:")
	w.configureEnvironments()

	return w.config, nil
}

// promptString prompts for a string value
func (w *SimpleWizard) promptString(label string, defaultValue string) string {
	fmt.Printf("%s [%s]: ", label, defaultValue)
	
	w.scanner.Scan()
	input := strings.TrimSpace(w.scanner.Text())
	
	if input == "" {
		return defaultValue
	}
	return input
}

// promptChoice prompts for a choice from options
func (w *SimpleWizard) promptChoice(label string, options []string, defaultValue string) string {
	fmt.Printf("%s (%s) [%s]: ", label, strings.Join(options, "/"), defaultValue)
	
	w.scanner.Scan()
	input := strings.TrimSpace(w.scanner.Text())
	
	if input == "" {
		return defaultValue
	}
	
	// Validate choice
	for _, opt := range options {
		if input == opt {
			return input
		}
	}
	
	fmt.Printf("Invalid choice. Using default: %s\n", defaultValue)
	return defaultValue
}

// promptYesNo prompts for a yes/no answer
func (w *SimpleWizard) promptYesNo(question string, defaultYes bool) bool {
	defaultStr := "n"
	if defaultYes {
		defaultStr = "y"
	}
	
	fmt.Printf("%s (y/n) [%s]: ", question, defaultStr)
	
	w.scanner.Scan()
	input := strings.TrimSpace(w.scanner.Text())
	
	if input == "" {
		return defaultYes
	}
	
	lower := strings.ToLower(input)
	return lower == "y" || lower == "yes"
}

// configureEnvironments configures environment settings
func (w *SimpleWizard) configureEnvironments() {
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
					fmt.Printf("\nConfiguring %s environment:\n", envName)
					env.UseSecretsManager = w.promptYesNo("Use AWS Secrets Manager?", true)
				}
				
				w.config.Environments[envName] = env
			}
			
			// Set default environment
			if len(w.config.Environments) > 0 {
				envNames := []string{}
				for name := range w.config.Environments {
					envNames = append(envNames, name)
				}
				w.config.DefaultEnvironment = w.promptChoice("Default environment", envNames, envNames[0])
			}
			return
		}
	}

	// Manual configuration
	fmt.Println("\nLet's configure your environments.")
	fmt.Println("Common environments: dev, staging, prod")
	
	// Add first environment
	envName := w.promptString("First environment name", "dev")
	w.addEnvironment(envName)
	w.config.DefaultEnvironment = envName

	// Ask if user wants to add more environments
	for {
		if !w.promptYesNo("\nAdd another environment?", false) {
			break
		}

		envName := w.promptString("Environment name", "")
		if envName != "" {
			w.addEnvironment(envName)
		}
	}
}

// addEnvironment adds a new environment configuration
func (w *SimpleWizard) addEnvironment(name string) {
	fmt.Printf("\nConfiguring environment: %s\n", name)

	env := config.Environment{
		Files: []string{fmt.Sprintf(".env.%s", name)},
		Path:  fmt.Sprintf("/%s/%s/", w.config.Project, name),
	}

	// Use Secrets Manager for this environment?
	if name == "prod" || name == "production" {
		env.UseSecretsManager = w.promptYesNo("Use AWS Secrets Manager?", true)
	} else {
		env.UseSecretsManager = w.promptYesNo("Use AWS Secrets Manager?", false)
	}

	w.config.Environments[name] = env
}