# envy - Environment Variable Management CLI

[![Build Status](https://github.com/drapon/envy/workflows/CI/badge.svg)](https://github.com/drapon/envy/actions)
[![Coverage Status](https://coveralls.io/repos/github/drapon/envy/badge.svg?branch=main)](https://coveralls.io/github/drapon/envy?branch=main)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.20+-blue.svg)](https://golang.org)
[![Release](https://img.shields.io/github/release/drapon/envy.svg)](https://github.com/drapon/envy/releases/latest)

`envy` is a CLI tool that simplifies environment variable management by syncing between local `.env` files and AWS Parameter Store/Secrets Manager.

## Features

- **Bidirectional Sync**: Push local `.env` files to AWS or pull from AWS to local
- **Multi-Environment Support**: Seamlessly manage development, staging, and production environments
- **Secure Storage**: Leverage AWS Parameter Store and Secrets Manager for secure variable storage
- **Smart Path Mapping**: Automatically map `.env` file types to AWS parameter hierarchies
- **Easy to Use**: Intuitive interface and simple commands
- **Flexible Configuration**: Configure once with `.envyrc`, use anywhere
- **High Performance**: Fast upload/download with parallel processing
- **Security First**: Automatic file permission settings and secure communication
- **Visual Feedback**: Color output and progress bars to track progress
- **Interactive Mode**: Interactive prompts with arrow key navigation

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap drapon/tap
brew install envy
```

### Go

```bash
go install github.com/drapon/envy@latest
```

### Download Binary

Download the latest version from the [releases page](https://github.com/drapon/envy/releases).

### Docker

```bash
docker pull drapon/envy:latest
```


## Quick Start

1. **Initialize your project**

   ```bash
   envy init
   ```

   Automatically detects existing `.env` files and initializes the project.

2. **Push to AWS**

   ```bash
   envy push
   ```

   Uploads variables from the default environment to AWS. First time use includes interactive overwrite confirmation.

3. **Pull from AWS**

   ```bash
   envy pull
   ```

   Downloads environment variables from AWS and saves them to local files.

4. **Work with other environments**
   ```bash
   envy push --env prod
   envy pull --env staging
   ```


## Usage

### Commands

- `envy init` - Initialize a new project
- `envy configure` - Interactive configuration wizard
- `envy push` - Upload local .env files to AWS
- `envy pull` - Download environment variables from AWS
- `envy list` - List available environment variables
- `envy diff` - Show differences between local and remote
- `envy run` - Run commands with injected environment variables
- `envy validate` - Validate environment variables
- `envy export` - Export environment variables in various formats
- `envy cache` - Manage cache


### Examples

```bash
# Initialize project with automatic .env file detection
envy init

# Push with interactive overwrite confirmation
envy push

# Push all environments at once
envy push --all

# Push with progress bar (force overwrite)
envy push --env prod --force

# Pull with automatic environment detection
envy pull

# Pull with backup
envy pull --env prod --backup

# Show differences between local and remote
envy diff --env staging

# List variables with color coding
envy list --env dev

# Run command with environment variables
envy run --env dev npm start

# Export as shell script
envy export --env prod --format shell > prod.sh

# Validate configuration
envy validate
```

### Key Features

- **Color Output**: Success in green, errors in red, warnings in yellow
- **Progress Bars**: Real-time progress display during push/pull operations
- **Interactive Overwrite Confirmation**: Select individually with arrow keys
- **Clean Log Output**: Show detailed logs with `--verbose` flag
- **Existing File Detection**: `init` command automatically detects existing `.env` files
- **Duplicate and Empty Value Checks**: Automatically detect and handle issues appropriately

## Configuration

Create a `.envyrc` file in your project root:

```yaml
project: myapp
default_environment: dev
aws:
  service: parameter_store # or secrets_manager
  region: ap-northeast-1
  profile: default
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
    use_secrets_manager: true
  production.local: # Supports environment names with dots
    files:
      - .env.production.local
    path: /myapp/production.local/
```

## AWS Permissions

Ensure your AWS credentials have the following permissions:

### Parameter Store

- `ssm:GetParameter`
- `ssm:GetParameters`
- `ssm:GetParametersByPath`
- `ssm:PutParameter`
- `ssm:DeleteParameter`

### Secrets Manager

- `secretsmanager:GetSecretValue`
- `secretsmanager:CreateSecret`
- `secretsmanager:UpdateSecret`
- `secretsmanager:DeleteSecret`
- `secretsmanager:ListSecrets`

### KMS (if using encryption)

- `kms:Decrypt`
- `kms:GenerateDataKey`


### Minimum Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter*",
        "ssm:PutParameter",
        "ssm:DeleteParameter",
        "secretsmanager:GetSecretValue",
        "secretsmanager:CreateSecret",
        "secretsmanager:UpdateSecret"
      ],
      "Resource": "*"
    }
  ]
}
```

## Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Configuration management by [Viper](https://github.com/spf13/viper)
- AWS integration using [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)

## Support

- [Documentation](https://github.com/drapon/envy/wiki)
- [Issue Tracker](https://github.com/drapon/envy/issues)
- [Discussions](https://github.com/drapon/envy/discussions)
- [Email](mailto:support@drapon.dev)

## Troubleshooting

### Common Issues and Solutions

1. **Cache Error**: `invalid cached config type`

   ```bash
   rm -rf ~/.envy/cache/*
   ```

2. **Environment Name Not Recognized**: Environment names with dots (e.g., `production.local`) are supported

3. **Permission Error**: Grant execute permission
   ```bash
   chmod +x envy
   ```

