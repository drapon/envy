# Use multi-stage build to minimize binary size
# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Optimize dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build-time environment variables
ARG VERSION="dev"
ARG COMMIT="unknown"
ARG BUILD_DATE="unknown"

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X github.com/drapon/envy/internal/version.Version=${VERSION} \
    -X github.com/drapon/envy/internal/version.GitCommit=${COMMIT} \
    -X github.com/drapon/envy/internal/version.BuildDate=${BUILD_DATE}" \
    -o envy ./cmd/envy

# Runtime stage
FROM scratch

# Metadata
LABEL org.opencontainers.image.title="envy" \
      org.opencontainers.image.description="Environment variables sync tool between local files and AWS Parameter Store/Secrets Manager" \
      org.opencontainers.image.vendor="yourusername" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/drapon/envy"

# Copy certificates and timezone info
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set non-root user
COPY --from=builder /etc/passwd /etc/passwd
USER nobody:nobody

# Copy binary
COPY --from=builder /build/envy /usr/local/bin/envy

# Working directory
WORKDIR /workspace

# Entrypoint
ENTRYPOINT ["/usr/local/bin/envy"]
CMD ["--help"]

# Health check (optional)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/envy", "version"]

# =============================================================================
# Development image (optional)
# =============================================================================
FROM golang:1.23-alpine AS development

# Install development tools
RUN apk add --no-cache \
    git \
    make \
    bash \
    curl \
    jq \
    aws-cli

# Install golangci-lint
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install air (hot reload)
RUN go install github.com/cosmtrek/air@latest

# Install mockgen
RUN go install github.com/golang/mock/mockgen@latest

WORKDIR /workspace

# Development environment variables
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Expose port (for metrics server)
EXPOSE 8080

# Start development server
CMD ["air", "-c", ".air.toml"]

# =============================================================================
# AWS Lambda image (optional)
# =============================================================================
FROM public.ecr.aws/lambda/provided:al2 AS lambda

# Copy binary
COPY --from=builder /build/envy /var/runtime/bootstrap

# Lambda environment variables
ENV ENVY_LAMBDA_MODE=true

# Entrypoint
ENTRYPOINT ["/var/runtime/bootstrap"]