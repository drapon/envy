# Contributing to envy

Thank you for your interest in contributing to envy! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Ways to Contribute](#ways-to-contribute)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)
- [Pull Requests](#pull-requests)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

All participants in this project are expected to follow these guidelines:

- **Be respectful**: Treat all participants with respect
- **Be constructive**: Provide constructive feedback rather than criticism
- **Be inclusive**: Respect diversity and welcome everyone
- **Be professional**: Maintain professional conduct

## Ways to Contribute

### Bug Reports
- Check existing issues first
- Include reproducible steps
- Provide environment information

### Feature Requests
- Explain the use case
- Describe expected behavior
- Provide implementation ideas if possible

### Code Contributions
- Bug fixes
- New features
- Performance improvements
- Refactoring

### Documentation
- Fix typos and errors
- Improve explanations
- Add examples
- Translate content

### Testing
- Add unit tests
- Add integration tests
- Improve test coverage

## Development Setup

### Prerequisites

- Go 1.20 or higher
- Git
- Make
- Docker (for integration tests)
- AWS CLI (optional)

### Setup Steps

1. **Fork the repository**
   
   Fork the envy repository on GitHub.

2. **Clone locally**
   ```bash
   git clone https://github.com/YOUR_USERNAME/envy.git
   cd envy
   ```

3. **Add upstream remote**
   ```bash
   git remote add upstream https://github.com/drapon/envy.git
   ```

4. **Install dependencies**
   ```bash
   go mod download
   ```

5. **Install development tools**
   ```bash
   # golangci-lint
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

   # gofumpt
   go install mvdan.cc/gofumpt@latest
   ```

6. **Verify build**
   ```bash
   make build
   ./envy --version
   ```

7. **Run tests**
   ```bash
   make test
   ```

## Development Workflow

### 1. Create a Branch

```bash
# Get latest changes
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/amazing-feature

# Create bug fix branch
git checkout -b fix/bug-description
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation only
- `refactor/` - Code refactoring
- `test/` - Test additions/changes
- `chore/` - Other changes

### 2. Make Changes

```bash
# Edit code
vim internal/aws/manager.go

# Format code
make fmt

# Run linter
make lint

# Run tests
make test
```

### 3. Commit Changes

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```bash
# Good examples
git commit -m "feat: add support for KMS encryption in Parameter Store"
git commit -m "fix: handle nil pointer in config loader"
git commit -m "docs: update installation guide for Windows users"
git commit -m "test: add unit tests for parallel upload"
git commit -m "refactor: simplify error handling in AWS client"
git commit -m "chore: update dependencies to latest versions"

# Bad examples
git commit -m "Fixed bug"
git commit -m "Update code"
git commit -m "WIP"
```

Commit message format:
```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only changes
- `style`: Changes that don't affect code meaning
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or modifying tests
- `chore`: Changes to build process or tools

### 4. Push Changes

```bash
git push origin feature/amazing-feature
```

## Coding Standards

### Go Standards

1. **Follow standard conventions**
   - [Effective Go](https://golang.org/doc/effective_go.html)
   - [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

2. **Formatting**
   ```bash
   # Use gofumpt
   make fmt
   ```

3. **Naming conventions**
   ```go
   // Package names: singular, lowercase
   package config

   // Exported types: start with uppercase
   type Manager struct {
       // ...
   }

   // Unexported types: start with lowercase
   type internalConfig struct {
       // ...
   }

   // Constants: CamelCase
   const DefaultTimeout = 30 * time.Second

   // Interfaces: end with -er
   type Loader interface {
       Load() error
   }
   ```

4. **Error handling**
   ```go
   // Return errors
   if err != nil {
       return fmt.Errorf("failed to load config: %w", err)
   }

   // Custom error types
   type ConfigError struct {
       Path string
       Err  error
   }

   func (e *ConfigError) Error() string {
       return fmt.Sprintf("config error at %s: %v", e.Path, e.Err)
   }
   ```

5. **Comments**
   ```go
   // Manager handles AWS Parameter Store operations.
   // It provides methods for pushing and pulling environment variables.
   type Manager struct {
       client ParameterStoreClient
       config *Config
   }

   // Push uploads local environment variables to AWS Parameter Store.
   // It returns an error if the upload fails.
   func (m *Manager) Push(ctx context.Context, vars map[string]string) error {
       // Implementation details...
   }
   ```

### Project-Specific Standards

1. **Package structure**
   ```
   cmd/        # Command line interfaces
   internal/   # Internal packages
   pkg/        # Public packages
   ```

2. **Dependency injection**
   ```go
   // Use interfaces
   type Storage interface {
       Get(key string) (string, error)
       Set(key string, value string) error
   }

   type Service struct {
       storage Storage
   }

   func NewService(storage Storage) *Service {
       return &Service{storage: storage}
   }
   ```

3. **Context usage**
   ```go
   // Use Context for all long-running operations
   func (m *Manager) Pull(ctx context.Context, prefix string) (map[string]string, error) {
       // ...
   }
   ```

## Testing

### Unit Tests

```go
// config_test.go
package config

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        want    *Config
        wantErr bool
    }{
        {
            name: "valid config",
            path: "testdata/valid.yaml",
            want: &Config{
                Project: "test-project",
                DefaultEnvironment: "dev",
            },
            wantErr: false,
        },
        {
            name:    "invalid path",
            path:    "testdata/nonexistent.yaml",
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := LoadConfig(tt.path)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Integration Tests

```go
//go:build integration

package integration

import (
    "testing"
    "github.com/drapon/envy/internal/testutil"
)

func TestE2EPushPull(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    // Use LocalStack
    client := testutil.NewLocalStackClient(t)
    
    // Test implementation...
}
```

### Test Coverage

```bash
# Run tests with coverage
make test-coverage

# Generate HTML report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Target coverage: 80% or higher

## Documentation

### Types of Documentation

1. **Code comments**
   - Use GoDoc format
   - Document all exported types and functions

2. **README.md**
   - Add new features
   - Update examples

3. **Command help**
   - Help text in `cmd/*/`
   - Add examples

### Documentation Guidelines

1. **Include executable examples**
   ```bash
   # Include working commands
   envy push --env dev --dry-run
   ```

2. **Keep it concise and clear**

3. **Update version information when applicable**

## Pull Requests

### Before Creating a PR

1. **Ensure tests pass**
   ```bash
   make test-all
   ```

2. **Ensure no lint errors**
   ```bash
   make lint
   ```

3. **Update documentation**

4. **Update CHANGELOG.md** (if applicable)

### PR Template

```markdown
## Summary
<!-- Brief description of changes -->

## Changes
<!-- List specific changes -->
- 
- 
- 

## Related Issues
<!-- Reference related issues -->
Fixes #123

## Testing
<!-- Describe testing approach -->
- [ ] Added/updated unit tests
- [ ] Added/updated integration tests
- [ ] Manual testing performed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review performed
- [ ] Comments added where necessary
- [ ] Documentation updated
- [ ] No breaking changes
- [ ] Dependencies updated in go.mod (if applicable)
```

### Review Process

1. **Automated checks**
   - Ensure CI/CD passes

2. **Code review**
   - At least one maintainer review
   - Address feedback

3. **Merge**
   - Squash and merge
   - Clean commit message

## Reporting Issues

### Bug Reports

```markdown
## Bug Description
<!-- Clear description of the bug -->

## Steps to Reproduce
1. 
2. 
3. 

## Expected Behavior
<!-- What should happen -->

## Actual Behavior
<!-- What actually happens -->

## Environment
- OS: [e.g., macOS 12.0]
- Go version: [e.g., 1.20]
- envy version: [e.g., v1.0.0]

## Logs
```
<!-- Relevant logs or error messages -->
```

## Screenshots
<!-- If applicable -->

## Additional Context
<!-- Any other relevant information -->
```

### Feature Requests

```markdown
## Feature Description
<!-- Clear description of the proposed feature -->

## Problem to Solve
<!-- Explain the problem this feature solves -->

## Proposed Solution
<!-- How you think it should be implemented -->

## Alternatives
<!-- Other approaches considered -->

## Additional Context
<!-- Any other relevant information -->
```

## Support

If you need help:

1. Check the [README](README.md)
2. Search existing [Issues](https://github.com/drapon/envy/issues)
3. Ask in [Discussions](https://github.com/drapon/envy/discussions)

## License

By contributing to envy, you agree that your contributions will be licensed under the MIT License.

## Acknowledgments

Thank you to all contributors! Your contributions make envy a better tool for everyone.

Contributors: [Contributors](https://github.com/drapon/envy/graphs/contributors)