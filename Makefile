.PHONY: all build clean test test-coverage test-integration test-all lint fmt install release help version embed-version version-bump-patch version-bump-minor version-bump-major prepare-release

# Variables
BINARY_NAME := envy
GO := go
GOFLAGS := -v
LDFLAGS := -s -w
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build variables
BUILD_DIR := ./dist
MAIN_PACKAGE := .
COVERAGE_FILE := coverage.out

# Go build flags with version information
LDFLAGS := -X 'github.com/drapon/envy/pkg/version.Version=$(VERSION)' \
           -X 'github.com/drapon/envy/pkg/version.BuildTime=$(BUILD_TIME)' \
           -X 'github.com/drapon/envy/pkg/version.GitCommit=$(GIT_COMMIT)'

# Default target
all: lint test build

# Build the binary
build: embed-version
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

# Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE)
	rm -rf .github/releases/
	$(GO) clean -cache

# Run tests
test:
	@echo "Running unit tests..."
	$(GO) test -v -race -timeout 5m ./cmd/... ./internal/... ./pkg/... ./test/...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic -timeout 5m ./cmd/... ./internal/... ./pkg/... ./test/...
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@$(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print "Total coverage: " $$3}'

# Run tests with coverage and check threshold
test-coverage-check: test-coverage
	@echo "Checking coverage threshold..."
	@coverage=$$($(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$coverage < 80" | bc) -eq 1 ]; then \
		echo "Coverage $$coverage% is below threshold 80%"; \
		exit 1; \
	else \
		echo "Coverage $$coverage% meets threshold"; \
	fi

# Run integration tests (requires LocalStack)
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v -race -tags=integration ./test/integration/...


# Run all tests
test-all: test test-integration

# LocalStack management
localstack-start:
	@echo "Starting LocalStack..."
	docker-compose -f docker-compose.test.yml up -d localstack
	@echo "Waiting for LocalStack to be ready..."
	@until docker-compose -f docker-compose.test.yml exec -T localstack curl -f http://localhost:4566/_localstack/health > /dev/null 2>&1; do \
		echo "Waiting for LocalStack..."; \
		sleep 2; \
	done
	@echo "LocalStack is ready!"

localstack-stop:
	@echo "Stopping LocalStack..."
	docker-compose -f docker-compose.test.yml down

localstack-logs:
	docker-compose -f docker-compose.test.yml logs -f localstack

# Run integration tests with LocalStack
test-integration-docker:
	@echo "Running integration tests with LocalStack..."
	docker-compose -f docker-compose.test.yml up --build test-runner

# Run tests in CI environment
test-ci: lint test-coverage test-integration-docker

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install -ldflags "$(LDFLAGS)"

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	gofmt -s -w .

# Lint code
lint:
	@echo "Linting code..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin; \
	fi
	golangci-lint run ./...

# Generate mocks
gen-mocks:
	@echo "Generating mocks..."
	@if ! command -v mockgen &> /dev/null; then \
		echo "Installing mockgen..."; \
		$(GO) install github.com/golang/mock/mockgen@latest; \
	fi
	$(GO) generate ./...

# Generate mocks for specific packages
gen-mocks-aws:
	@echo "Generating AWS mocks..."
	mockgen -source=internal/aws/client/client.go -destination=internal/testutil/mocks_aws_client.go -package=testutil

# Run unit tests with verbose output
test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -race -short ./...

# Run specific package tests
test-config:
	@echo "Running config package tests..."
	$(GO) test -v -race ./internal/config/...

test-env:
	@echo "Running env package tests..."
	$(GO) test -v -race ./internal/env/...

test-aws:
	@echo "Running AWS package tests..."
	$(GO) test -v -race ./internal/aws/...

test-cmd:
	@echo "Running command tests..."
	$(GO) test -v -race ./cmd/...

# Run tests with coverage for specific packages
test-coverage-config:
	@echo "Running config tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage-config.out ./internal/config/...
	$(GO) tool cover -html=coverage-config.out -o coverage-config.html

test-coverage-env:
	@echo "Running env tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage-env.out ./internal/env/...
	$(GO) tool cover -html=coverage-env.out -o coverage-env.html

test-coverage-aws:
	@echo "Running AWS tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage-aws.out ./internal/aws/...
	$(GO) tool cover -html=coverage-aws.out -o coverage-aws.html

# Run benchmarks
test-bench:
	@echo "Running benchmarks..."
	$(GO) test -v -bench=. -benchmem ./...

# Run benchmarks for specific packages
test-bench-config:
	@echo "Running config benchmarks..."
	$(GO) test -v -bench=. -benchmem ./internal/config/...

test-bench-env:
	@echo "Running env benchmarks..."
	$(GO) test -v -bench=. -benchmem ./internal/env/...

# Run race detection tests
test-race:
	@echo "Running race detection tests..."
	$(GO) test -race ./...

# Run tests with CPU profiling
test-profile-cpu:
	@echo "Running tests with CPU profiling..."
	$(GO) test -cpuprofile=cpu.prof -bench=. ./...

# Run tests with memory profiling
test-profile-mem:
	@echo "Running tests with memory profiling..."
	$(GO) test -memprofile=mem.prof -bench=. ./...

# Clean test artifacts
test-clean:
	@echo "Cleaning test artifacts..."
	rm -f coverage*.out coverage*.html
	rm -f cpu.prof mem.prof
	rm -f *.test

# Run parallel tests
test-parallel:
	@echo "Running tests in parallel..."
	$(GO) test -v -race -parallel 4 ./...

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GO) run $(MAIN_PACKAGE)

# Create a release build
release: clean embed-version build-all
	@echo "Creating release builds..."
	@mkdir -p $(BUILD_DIR)

	# Create release archives
	@echo "Creating release archives..."
	@for file in $(BUILD_DIR)/$(BINARY_NAME)-*; do \
		base=$$(basename $$file); \
		if [[ "$$base" == *".exe" ]]; then \
			zip -j $(BUILD_DIR)/$${base%.exe}.zip $$file; \
		else \
			tar -czf $(BUILD_DIR)/$$base.tar.gz -C $(BUILD_DIR) $$base; \
		fi; \
	done

	# Create checksums
	@echo "Creating checksums..."
	cd $(BUILD_DIR) && sha256sum *.tar.gz *.zip > checksums.txt

	@echo "Release builds created in $(BUILD_DIR)"

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

# Development setup
dev-setup:
	@echo "Setting up development environment..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/goreleaser/goreleaser@latest
	@echo "Development environment setup complete"

# Release using GoReleaser
release-goreleaser:
	@echo "Running GoReleaser..."
	@if ! command -v goreleaser &> /dev/null; then \
		echo "goreleaser not found. Installing..."; \
		go install github.com/goreleaser/goreleaser@latest; \
	fi
	goreleaser release --clean

# GoReleaser snapshot (development build)
release-snapshot:
	@echo "Creating snapshot release..."
	@if ! command -v goreleaser &> /dev/null; then \
		echo "goreleaser not found. Installing..."; \
		go install github.com/goreleaser/goreleaser@latest; \
	fi
	goreleaser release --snapshot --clean

# Validate release configuration (dry-run)
release-check:
	@echo "Checking release configuration..."
	@if ! command -v goreleaser &> /dev/null; then \
		echo "goreleaser not found. Installing..."; \
		go install github.com/goreleaser/goreleaser@latest; \
	fi
	goreleaser check

# Sign binaries
sign-binaries:
	@echo "Signing binaries..."
	./scripts/sign-release.sh --platform all --version $(VERSION) --dist-dir $(BUILD_DIR)

# Push Docker image
docker-push: docker-build
	@echo "Pushing Docker image..."
	docker tag $(BINARY_NAME):$(VERSION) ghcr.io/drapon/$(BINARY_NAME):$(VERSION)
	docker tag $(BINARY_NAME):$(VERSION) ghcr.io/drapon/$(BINARY_NAME):latest
	docker push ghcr.io/drapon/$(BINARY_NAME):$(VERSION)
	docker push ghcr.io/drapon/$(BINARY_NAME):latest

# Build multi-architecture Docker image
docker-buildx:
	@echo "Building multi-arch Docker image..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--tag ghcr.io/drapon/$(BINARY_NAME):$(VERSION) \
		--tag ghcr.io/drapon/$(BINARY_NAME):latest \
		--push .

# Update Homebrew formula
update-homebrew:
	@echo "Updating Homebrew formula..."
	@echo "This is typically done automatically by GoReleaser"
	@echo "Manual update: edit homebrew/envy.rb with new version and checksums"

# Generate release notes
generate-release-notes:
	@echo "Generating release notes..."
	@if [ -f .github/release-template.md ]; then \
		echo "Using template from .github/release-template.md"; \
		cat .github/release-template.md | sed "s/{{ .Tag }}/$(VERSION)/g" | sed "s/{{ .Date }}/$(shell date -u '+%Y-%m-%d')/g"; \
	else \
		echo "No release template found"; \
	fi

# Show help
help:
	@echo "Available targets:"
	@echo "  all                   - Run lint, test, and build"
	@echo "  build                 - Build the binary for current platform"
	@echo "  build-all             - Build for all platforms"
	@echo "  clean                 - Remove build artifacts"
	@echo "  deps                  - Install dependencies"
	@echo "  deps-update           - Update dependencies"
	@echo "  dev-setup             - Setup development environment"
	@echo "  docker-build          - Build Docker image"
	@echo "  docker-push           - Push Docker image to registry"
	@echo "  docker-buildx         - Build multi-arch Docker image"
	@echo "  fmt                   - Format code"
	@echo "  gen-mocks             - Generate mocks"
	@echo "  generate-release-notes - Generate release notes from template"
	@echo "  help                  - Show this help message"
	@echo "  install               - Install the binary"
	@echo "  lint                  - Run linters"
	@echo "  release               - Create release builds"
	@echo "  release-goreleaser    - Create release using GoReleaser"
	@echo "  release-snapshot      - Create snapshot release (dev build)"
	@echo "  release-check         - Check release configuration"
	@echo "  run                   - Run the application"
	@echo "  sign-binaries         - Sign release binaries"
	@echo "  test                  - Run unit tests"
	@echo "  test-coverage         - Run tests with coverage"
	@echo "  test-integration      - Run integration tests"
	@echo "  test-all              - Run all tests"
	@echo "  test-unit             - Run unit tests"
	@echo "  test-config           - Run config package tests"
	@echo "  test-env              - Run env package tests"
	@echo "  test-aws              - Run AWS package tests"
	@echo "  test-cmd              - Run command tests"
	@echo "  test-coverage-*       - Run tests with coverage for specific packages"
	@echo "  test-bench            - Run benchmarks"
	@echo "  test-bench-*          - Run benchmarks for specific packages"
	@echo "  test-race             - Run race detection tests"
	@echo "  test-parallel         - Run tests in parallel"
	@echo "  test-profile-*        - Run tests with profiling"
	@echo "  test-clean            - Clean test artifacts"
	@echo "  test-integration-docker - Run integration tests with LocalStack in Docker"
	@echo "  test-ci               - Run tests in CI environment"
	@echo "  update-homebrew       - Update Homebrew formula"
	@echo "  version               - Show current version"
	@echo "  version-bump-patch    - Bump patch version"
	@echo "  version-bump-minor    - Bump minor version"
	@echo "  version-bump-major    - Bump major version"
	@echo "  embed-version         - Embed version information"
	@echo "  prepare-release       - Prepare for release"
	@echo "  localstack-start      - Start LocalStack container"
	@echo "  localstack-stop       - Stop LocalStack container"
	@echo "  localstack-logs       - Show LocalStack logs"
	@echo "  gen-mocks             - Generate all mocks"
	@echo "  gen-mocks-aws         - Generate AWS mocks"

# Version management
version: ## Show current version
	@./scripts/version.sh current

version-bump-patch: ## Bump patch version
	@./scripts/version.sh bump patch

version-bump-minor: ## Bump minor version
	@./scripts/version.sh bump minor

version-bump-major: ## Bump major version
	@./scripts/version.sh bump major

embed-version: ## Embed version information
	@./scripts/version.sh embed

# Prepare for release
prepare-release: ## Prepare for release
	@./scripts/prepare-release.sh