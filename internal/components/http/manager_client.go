package http

import (
	"context"
	"time"
)

// ManagerClient Manager endpoint dedicated HTTP client
type ManagerClient struct {
	client *Client
}

// ManagerClientConfig Manager client configuration
type ManagerClientConfig struct {
	BaseURL    string        // Manager endpoint address
	AuthToken  string        // authentication token (optional)
	Timeout    time.Duration // request timeout
	MaxRetries int           // maximum retry count
}

// NewManagerClient create Manager endpoint HTTP client
func NewManagerClient(cfg ManagerClientConfig) *ManagerClient {
	client := NewClient(ClientConfig{
		BaseURL:    cfg.BaseURL,
		AuthToken:  cfg.AuthToken,
		Timeout:    cfg.Timeout,
		MaxRetries: cfg.MaxRetries,
	})

	return &ManagerClient{
		client: client,
	}
}

// DoRequest execute HTTP request (wrapper for client's DoRequest)
func (m *ManagerClient) DoRequest(ctx context.Context, opts RequestOptions) error {
	return m.client.DoRequest(ctx, opts)
}

// DoRequestRaw execute HTTP request and return raw response
func (m *ManagerClient) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	return m.client.DoRequestRaw(ctx, opts)
}
