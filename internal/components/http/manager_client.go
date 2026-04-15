package http

import (
	"context"
	"time"
)

// ManagerClient Managerafterendpoint专useHTTPclient-side
type ManagerClient struct {
	client *Client
}

// ManagerClientConfig Managerclient-sideconfig
type ManagerClientConfig struct {
	BaseURL   string        // Managerafterendpointaddress
	AuthToken string        // authenticateToken（optional）
	Timeout   time.Duration // requesttimeouttime
	MaxRetries int          // maximumretrytimescount
}

// NewManagerClient createManagerafterendpointHTTPclient-side
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

// DoRequest executeHTTPrequest（encapsulation通useclient-sideofDoRequest）
func (m *ManagerClient) DoRequest(ctx context.Context, opts RequestOptions) error {
	return m.client.DoRequest(ctx, opts)
}

// DoRequestRaw executeHTTPrequestandreturnoriginalrespond
func (m *ManagerClient) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	return m.client.DoRequestRaw(ctx, opts)
}

