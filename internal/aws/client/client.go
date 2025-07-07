package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Client represents an AWS client wrapper
type Client struct {
	config        aws.Config
	ssmClient     *ssm.Client
	secretsClient *secretsmanager.Client
	region        string
	profile       string
	mu            sync.Mutex
}

// Options for creating a new AWS client
type Options struct {
	Region  string
	Profile string
}

// NewClient creates a new AWS client
func NewClient(ctx context.Context, opts Options) (*Client, error) {
	// Validate region
	if opts.Region == "" {
		return nil, fmt.Errorf("AWS region is required")
	}

	// Load AWS configuration
	configOpts := []func(*config.LoadOptions) error{
		config.WithRegion(opts.Region),
	}

	// Add profile if specified
	if opts.Profile != "" && opts.Profile != "default" {
		configOpts = append(configOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		config:  cfg,
		region:  opts.Region,
		profile: opts.Profile,
	}, nil
}

// SSM returns the SSM (Parameter Store) client
func (c *Client) SSM() *ssm.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ssmClient == nil {
		c.ssmClient = ssm.NewFromConfig(c.config)
	}
	return c.ssmClient
}

// SecretsManager returns the Secrets Manager client
func (c *Client) SecretsManager() *secretsmanager.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.secretsClient == nil {
		c.secretsClient = secretsmanager.NewFromConfig(c.config)
	}
	return c.secretsClient
}

// Region returns the configured AWS region
func (c *Client) Region() string {
	return c.region
}

// Profile returns the configured AWS profile
func (c *Client) Profile() string {
	return c.profile
}

// Config returns the underlying AWS config
func (c *Client) Config() aws.Config {
	return c.config
}
