//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

// LocalStackHelper is a helper for LocalStack environment
type LocalStackHelper struct {
	endpoint string
	region   string
}

// NewLocalStackHelper creates a new LocalStack helper
func NewLocalStackHelper() *LocalStackHelper {
	endpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	return &LocalStackHelper{
		endpoint: endpoint,
		region:   region,
	}
}

// IsRunning checks if LocalStack is running
func (h *LocalStackHelper) IsRunning() bool {
	// Attempt to connect to the endpoint
	conn, err := net.DialTimeout("tcp", "localhost:4566", 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetEndpoint returns the LocalStack endpoint
func (h *LocalStackHelper) GetEndpoint() string {
	return h.endpoint
}

// Cleanup cleans up the LocalStack environment
func (h *LocalStackHelper) Cleanup(ctx context.Context) error {
	// In the current implementation, cleanup is done through envy's Manager,
	// so no special processing is needed here
	return nil
}

// WaitForLocalStack waits until LocalStack is fully started
func (h *LocalStackHelper) WaitForLocalStack(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if h.IsRunning() {
			// Wait a bit for services to fully start
			time.Sleep(2 * time.Second)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("LocalStack did not become ready within %v", timeout)
}

// GetLocalStackStatus gets LocalStack status information
func (h *LocalStackHelper) GetLocalStackStatus() map[string]interface{} {
	status := map[string]interface{}{
		"endpoint": h.endpoint,
		"region":   h.region,
		"running":  h.IsRunning(),
	}

	return status
}
