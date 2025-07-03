#!/bin/bash
# LocalStack initialization script - Test data setup

set -e

echo "Setting up test data in LocalStack..."

# AWS CLI alias
AWS="aws --endpoint-url=http://localhost:4566"

# Create KMS key for test (for encrypted parameters)
echo "Creating KMS key for test..."
KMS_KEY_ID=$($AWS kms create-key --description "envy-test-key" --query 'KeyMetadata.KeyId' --output text)
$AWS kms create-alias --alias-name alias/envy-test --target-key-id $KMS_KEY_ID

# Create test parameters in Parameter Store
echo "Creating test parameters in Parameter Store..."

# Basic parameters
$AWS ssm put-parameter \
    --name "/envy-test/common/DATABASE_URL" \
    --value "postgresql://localhost:5432/test" \
    --type "String" \
    --overwrite

$AWS ssm put-parameter \
    --name "/envy-test/common/API_ENDPOINT" \
    --value "https://api.test.local" \
    --type "String" \
    --overwrite

# Encrypted parameters
$AWS ssm put-parameter \
    --name "/envy-test/common/SECRET_KEY" \
    --value "super-secret-test-key" \
    --type "SecureString" \
    --key-id "alias/envy-test" \
    --overwrite

# Environment-specific parameters
for env in dev staging prod; do
    $AWS ssm put-parameter \
        --name "/envy-test/${env}/APP_ENV" \
        --value "${env}" \
        --type "String" \
        --overwrite
        
    $AWS ssm put-parameter \
        --name "/envy-test/${env}/LOG_LEVEL" \
        --value $([ "$env" = "prod" ] && echo "error" || echo "debug") \
        --type "String" \
        --overwrite
done

# Create test secrets in Secrets Manager
echo "Creating test secrets in Secrets Manager..."

# Basic secret
$AWS secretsmanager create-secret \
    --name "envy-test/database-credentials" \
    --secret-string '{"username":"testuser","password":"testpass123"}' \
    || $AWS secretsmanager update-secret \
        --secret-id "envy-test/database-credentials" \
        --secret-string '{"username":"testuser","password":"testpass123"}'

# Environment variable format secret
$AWS secretsmanager create-secret \
    --name "envy-test/api-keys" \
    --secret-string '{"API_KEY":"test-api-key-12345","API_SECRET":"test-api-secret-67890"}' \
    || $AWS secretsmanager update-secret \
        --secret-id "envy-test/api-keys" \
        --secret-string '{"API_KEY":"test-api-key-12345","API_SECRET":"test-api-secret-67890"}'

echo "Test data setup completed!"

# Verify test data
echo "Verifying test data..."
echo "Parameter Store parameters:"
$AWS ssm get-parameters-by-path --path "/envy-test" --recursive --query 'Parameters[*].Name'

echo "Secrets Manager secrets:"
$AWS secretsmanager list-secrets --query 'SecretList[?starts_with(Name, `envy-test/`)].Name'