package http

import "time"

// ClientConfig HTTP client configuration
type ClientConfig struct {
	BaseURL    string        // base URL
	AuthToken  string        // authentication token (optional)
	Timeout    time.Duration // request timeout
	MaxRetries int           // maximum retry count (default 3)
}

// RequestOptions request options
type RequestOptions struct {
	Method      string            // HTTP method
	Path        string            // request path
	QueryParams map[string]string // query parameters
	Headers     map[string]string // custom request headers
	Body        interface{}       // request body (will be automatically serialized to JSON)
	Response    interface{}       // response body (will be automatically deserialized)
}
