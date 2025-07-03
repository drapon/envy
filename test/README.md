# envy Test Guide

This directory contains integration tests and end-to-end tests for the envy project.

## Test Structure

```
test/
├── integration/          # Integration tests
│   ├── aws_integration_test.go    # Integration tests with AWS services
│   └── localstack_helper.go       # LocalStack helper
│   ├── scenarios_test.go          # Complete workflow tests
│   └── cli_test.go               # CLI command tests
└── localstack-init/     # LocalStack initialization scripts
    └── 01-setup-test-data.sh     # Test data setup
```

## Prerequisites

### Integration Tests

- Go 1.20 or higher
- Docker and Docker Compose
- Make

### E2E Tests

- All of the above plus envy binary must be built

## Running Tests

### 1. Unit Tests Only

```bash
make test
```

### 2. Integration Tests (Using LocalStack)

#### Method 1: Using Make Commands

```bash
# Start LocalStack
make localstack-start

# Run integration tests
make test-integration

# Stop LocalStack
make localstack-stop
```

#### Method 2: Using Docker Compose

```bash
# Run everything with Docker
make test-integration-docker
```

### 3. E2E Tests

```bash
# Run E2E tests (automatically builds binary)
```

### 4. All Tests

```bash
# Run unit tests, integration tests, and E2E tests
make test-all
```

## LocalStack Management

### Starting

```bash
docker-compose -f docker-compose.test.yml up -d localstack
```

### Status Check

```bash
# Health check
curl http://localhost:4566/_localstack/health

# Log check
docker-compose -f docker-compose.test.yml logs -f localstack
```

### Stopping

```bash
docker-compose -f docker-compose.test.yml down
```

## Test Environment Variables

### Integration Tests

```bash
# LocalStack endpoint (default: http://localhost:4566)
export LOCALSTACK_ENDPOINT=http://localhost:4566

# AWS region (default: us-east-1)
export AWS_REGION=us-east-1

# Use real AWS (Warning: this will incur costs)
export TEST_REAL_AWS=true
```

### E2E Tests

```bash
# Enable E2E tests (required)
export ENVY_E2E_TESTS=true

# Path to envy binary (optional)
export ENVY_BIN=/path/to/envy

# Enable interactive mode tests (optional)
export TEST_INTERACTIVE=true
```

## CI/CD Execution

GitHub Actions example:

```yaml
- name: Start LocalStack
make localstack-start

- name: Run Integration Tests
make test-integration
  env:
    AWS_ENDPOINT_URL: http://localhost:4566
    AWS_ACCESS_KEY_ID: test
    AWS_SECRET_ACCESS_KEY: test

- name: Run E2E Tests
  env:
    ENVY_E2E_TESTS: true
```

## Troubleshooting

### LocalStack Won't Start

1. Check if Docker is running
   ```bash
   docker ps
   ```

2. Check if port 4566 is available
   ```bash
   lsof -i :4566
   ```

3. Check LocalStack logs
   ```bash
   make localstack-logs
   ```

### Tests Timeout

1. Verify LocalStack is running properly
2. Increase test timeout value
   ```bash
   ```

### AWS Credentials Error

When using LocalStack, set these environment variables:

```bash
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_ENDPOINT_URL=http://localhost:4566
```

## Adding Tests

### Adding New Integration Tests

1. Create a new test file in `test/integration/` directory
2. Add build tag: `//go:build integration`
3. Implement test using `AWSIntegrationTestSuite`

### Adding New E2E Tests

1. Create a new test file in appropriate directory
2. Add build tag if needed
3. Implement test using `E2EScenarioTestSuite`

## Best Practices

1. **Test Independence**: Each test should not depend on other tests
2. **Cleanup**: Always clean up resources after tests
3. **Timeouts**: Set appropriate timeouts
4. **Error Handling**: Always test error cases
5. **Parallel Execution**: Make tests runnable in parallel when possible

## Debugging

### Detailed Log Output

```bash
# Go test verbose logs
go test -v ./test/integration/...

# envy debug mode
ENVY_DEBUG=true make test-integration```

### Running Specific Tests

```bash
# Run specific test function
go test -v -run TestParameterStoreOperations ./test/integration/...

# Run specific subtest
go test -v -run TestCompleteWorkflow/1._Project_Initialization ./test/integration/...
```