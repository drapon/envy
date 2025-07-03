package export

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/drapon/envy/cmd/root"
	"github.com/drapon/envy/internal/aws"
	"github.com/drapon/envy/internal/config"
	"github.com/drapon/envy/internal/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	environment string
	format      string
	output      string
	name        string
	namespace   string
	from        string
	filter      string
	exclude     string
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export environment variables in various formats",
	Long: `Export environment variables in different formats for use with
other tools and systems.

This command can export variables as shell scripts, Docker env files,
Kubernetes ConfigMaps, or other supported formats.`,
	Example: `  # Export as shell script
  envy export --env production --format shell
  
  # Export as Docker env file
  envy export --env production --format docker --output docker.env
  
  # Export as Kubernetes ConfigMap
  envy export --env production --format k8s-configmap --name myapp-config
  
  # Export as Kubernetes Secret
  envy export --env production --format k8s-secret --name myapp-secret
  
  # Export as GitHub Actions secrets
  envy export --env production --format github-actions
  
  # Export as JSON
  envy export --env production --format json --output config.json
  
  # Export specific variables only
  envy export --env production --filter "API_*"
  
  # Export excluding certain variables
  envy export --env production --exclude "SECRET_*"`,
	RunE: runExport,
}

func init() {
	root.GetRootCmd().AddCommand(exportCmd)

	// Add flags specific to export command
	exportCmd.Flags().StringVarP(&environment, "env", "e", "", "Environment to export")
	exportCmd.Flags().StringVarP(&format, "format", "f", "shell", "Export format (shell/docker/k8s-configmap/k8s-secret/github-actions/json/yaml)")
	exportCmd.Flags().StringVarP(&output, "output", "o", "", "Output file (stdout if not specified)")
	exportCmd.Flags().StringVarP(&name, "name", "n", "", "Resource name (for k8s exports)")
	exportCmd.Flags().String("namespace", "default", "Kubernetes namespace")
	exportCmd.Flags().String("from", "local", "Source (local/aws)")
	exportCmd.Flags().String("filter", "", "Filter pattern for variables to export")
	exportCmd.Flags().String("exclude", "", "Pattern for variables to exclude")

	// Bind namespace flag to viper
	viper.BindPFlag("export.namespace", exportCmd.Flags().Lookup("namespace"))
	namespace = viper.GetString("export.namespace")
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Use default environment if not specified
	if environment == "" {
		environment = cfg.DefaultEnvironment
	}

	// Get environment file
	var envFile *env.File
	if from == "aws" {
		// Pull from AWS
		envFile, err = pullFromAWS(ctx, cfg, environment)
		if err != nil {
			return fmt.Errorf("failed to pull from AWS: %w", err)
		}
	} else {
		// Load from local files
		envFile, err = loadLocalFiles(cfg, environment)
		if err != nil {
			return fmt.Errorf("failed to load local files: %w", err)
		}
	}

	// Apply filters
	envFile = applyFilters(envFile, filter, exclude)

	// Validate required parameters for specific formats
	if (format == "k8s-configmap" || format == "k8s-secret") && name == "" {
		return fmt.Errorf("--name is required for %s format", format)
	}

	// Export in the requested format
	var writer io.Writer = os.Stdout
	if output != "" {
		file, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
	}

	switch format {
	case "shell":
		err = exportShell(writer, envFile)
	case "docker":
		err = exportDocker(writer, envFile)
	case "k8s-configmap":
		err = exportK8sConfigMap(writer, envFile, name, namespace)
	case "k8s-secret":
		err = exportK8sSecret(writer, envFile, name, namespace)
	case "github-actions":
		err = exportGitHubActions(writer, envFile)
	case "json":
		err = exportJSON(writer, envFile)
	case "yaml":
		err = exportYAML(writer, envFile)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	if output != "" {
		fmt.Printf("Successfully exported to %s\n", output)
	}

	return nil
}

func pullFromAWS(ctx context.Context, cfg *config.Config, envName string) (*env.File, error) {
	awsManager, err := aws.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS manager: %w", err)
	}

	return awsManager.PullEnvironment(ctx, envName)
}

func loadLocalFiles(cfg *config.Config, envName string) (*env.File, error) {
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	if len(envConfig.Files) == 0 {
		return nil, fmt.Errorf("no files configured for environment %s", envName)
	}

	manager := env.NewManager(".")
	return manager.LoadFiles(envConfig.Files)
}

func applyFilters(envFile *env.File, filterPattern, excludePattern string) *env.File {
	if filterPattern == "" && excludePattern == "" {
		return envFile
	}

	result := env.NewFile()

	for _, key := range envFile.Keys() {
		include := true

		// Apply filter pattern
		if filterPattern != "" {
			matched, err := regexp.MatchString(filterPattern, key)
			if err != nil || !matched {
				include = false
			}
		}

		// Apply exclude pattern
		if excludePattern != "" && include {
			matched, err := regexp.MatchString(excludePattern, key)
			if err == nil && matched {
				include = false
			}
		}

		if include {
			if value, ok := envFile.Get(key); ok {
				result.Set(key, value)
			}
		}
	}

	return result
}

func exportShell(w io.Writer, envFile *env.File) error {
	fmt.Fprintln(w, "#!/bin/bash")
	fmt.Fprintln(w, "# Generated by envy")
	fmt.Fprintln(w, "# Run: source <filename> or eval $(envy export)")
	fmt.Fprintln(w)

	for _, key := range envFile.SortedKeys() {
		value, _ := envFile.Get(key)
		// Escape single quotes in value
		escapedValue := strings.ReplaceAll(value, "'", "'\"'\"'")
		fmt.Fprintf(w, "export %s='%s'\n", key, escapedValue)
	}

	return nil
}

func exportDocker(w io.Writer, envFile *env.File) error {
	fmt.Fprintln(w, "# Generated by envy")
	fmt.Fprintln(w, "# Add to Dockerfile: COPY <filename> /.env")
	fmt.Fprintln(w, "# Or use with docker run: --env-file <filename>")
	fmt.Fprintln(w)

	for _, key := range envFile.SortedKeys() {
		value, _ := envFile.Get(key)
		fmt.Fprintf(w, "%s=%s\n", key, value)
	}

	return nil
}

func exportK8sConfigMap(w io.Writer, envFile *env.File, name, namespace string) error {
	configMap := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"data": envFile.ToMap(),
	}

	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(configMap)
}

func exportK8sSecret(w io.Writer, envFile *env.File, name, namespace string) error {
	// Encode all values to base64
	data := make(map[string]string)
	for key, value := range envFile.ToMap() {
		data[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	secret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"type": "Opaque",
		"data": data,
	}

	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(secret)
}

func exportGitHubActions(w io.Writer, envFile *env.File) error {
	fmt.Fprintln(w, "#!/bin/bash")
	fmt.Fprintln(w, "# Generated by envy")
	fmt.Fprintln(w, "# Run this script to set GitHub Actions secrets")
	fmt.Fprintln(w, "# Requires: gh CLI tool to be installed and authenticated")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "set -e")
	fmt.Fprintln(w)

	for _, key := range envFile.SortedKeys() {
		value, _ := envFile.Get(key)
		// Escape for shell
		escapedValue := strings.ReplaceAll(value, "'", "'\"'\"'")
		fmt.Fprintf(w, "gh secret set %s --body '%s'\n", key, escapedValue)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "echo 'All secrets have been set successfully!'")

	return nil
}

func exportJSON(w io.Writer, envFile *env.File) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(envFile.ToMap())
}

func exportYAML(w io.Writer, envFile *env.File) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(envFile.ToMap())
}
