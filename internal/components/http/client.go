package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client generic HTTP client
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
	maxRetries int
}

// NewClient create new HTTP client
func NewClient(cfg ClientConfig) *Client {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1 // default retry 3 times
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:    cfg.BaseURL,
		authToken:  cfg.AuthToken,
		maxRetries: cfg.MaxRetries,
	}
}

// DoRequest execute HTTP request
func (c *Client) DoRequest(ctx context.Context, opts RequestOptions) error {
	return c.doRequestOnce(ctx, opts)
}

// doRequestOnce execute single HTTP request
func (c *Client) doRequestOnce(ctx context.Context, opts RequestOptions) error {
	// build URL
	reqURL := c.baseURL + opts.Path

	// add query parameters
	if len(opts.QueryParams) > 0 {
		params := url.Values{}
		for k, v := range opts.QueryParams {
			params.Set(k, v)
		}
		reqURL += "?" + params.Encode()
	}

	// build request body
	var bodyReader io.Reader
	if opts.Body != nil {
		data, err := json.Marshal(opts.Body)
		if err != nil {
			return fmt.Errorf("serialize request body failed: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// set default request header
	req.Header.Set("Content-Type", "application/json")

	// set authentication token
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// set custom request headers
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	// send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// check HTTP status code
	/*if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}*/

	// parse response body
	if opts.Response != nil {
		if err := json.Unmarshal(body, opts.Response); err != nil {
			return fmt.Errorf("parse response failed: %w, response body: %s", err, string(body))
		}
	}

	return nil
}

// DoRequestRaw execute HTTP request and return raw response (no automatic JSON parsing)
func (c *Client) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	var responseBody []byte
	var err error

	operation := func() error {
		// build URL
		reqURL := c.baseURL + opts.Path

		// add query parameters
		if len(opts.QueryParams) > 0 {
			params := url.Values{}
			for k, v := range opts.QueryParams {
				params.Set(k, v)
			}
			reqURL += "?" + params.Encode()
		}

		// build request body
		var bodyReader io.Reader
		if opts.Body != nil {
			data, marshalErr := json.Marshal(opts.Body)
			if marshalErr != nil {
				return fmt.Errorf("serialize request body failed: %w", marshalErr)
			}
			bodyReader = bytes.NewReader(data)
		}

		// create HTTP request
		req, createErr := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
		if createErr != nil {
			return fmt.Errorf("failed to create request: %w", createErr)
		}

		// set default request header
		req.Header.Set("Content-Type", "application/json")

		// set authentication token
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}

		// set custom request headers
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}

		// send request
		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("request failed: %w", doErr)
		}
		defer resp.Body.Close()

		// read response body
		responseBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// check HTTP status code
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody))
		}

		return nil
	}

	if err := operation(); err != nil {
		return nil, err
	}

	return responseBody, nil
}
