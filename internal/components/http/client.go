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

// Client 通useHTTPclient-side
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
	maxRetries int
}

// NewClient create newHTTPclient-side
func NewClient(cfg ClientConfig) *Client {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1 // defaultretry3times
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

// DoRequest executeHTTPrequest
func (c *Client) DoRequest(ctx context.Context, opts RequestOptions) error {
	return c.doRequestOnce(ctx, opts)
}

// doRequestOnce execute单timesHTTPrequest
func (c *Client) doRequestOnce(ctx context.Context, opts RequestOptions) error {
	// buildURL
	reqURL := c.baseURL + opts.Path

	// addqueryparameter
	if len(opts.QueryParams) > 0 {
		params := url.Values{}
		for k, v := range opts.QueryParams {
			params.Set(k, v)
		}
		reqURL += "?" + params.Encode()
	}

	// buildrequestbody
	var bodyReader io.Reader
	if opts.Body != nil {
		data, err := json.Marshal(opts.Body)
		if err != nil {
			return fmt.Errorf("serializerequestbodyfailed: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// createHTTPrequest
	req, err := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// setdefaultrequest header
	req.Header.Set("Content-Type", "application/json")

	// setauthenticateToken
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// set自定义request header
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	// sendrequest
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("requestfailed: %w", err)
	}
	defer resp.Body.Close()

	// readrespondbody
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// inspectHTTPstate码
	/*if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}*/

	// parserespondbody
	if opts.Response != nil {
		if err := json.Unmarshal(body, opts.Response); err != nil {
			return fmt.Errorf("parserespondfailed: %w, respondbody: %s", err, string(body))
		}
	}

	return nil
}

// DoRequestRaw executeHTTPrequestandreturnoriginalrespond（noautomaticparseJSON）
func (c *Client) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	var responseBody []byte
	var err error

	operation := func() error {
		// buildURL
		reqURL := c.baseURL + opts.Path

		// addqueryparameter
		if len(opts.QueryParams) > 0 {
			params := url.Values{}
			for k, v := range opts.QueryParams {
				params.Set(k, v)
			}
			reqURL += "?" + params.Encode()
		}

		// buildrequestbody
		var bodyReader io.Reader
		if opts.Body != nil {
			data, marshalErr := json.Marshal(opts.Body)
			if marshalErr != nil {
				return fmt.Errorf("serializerequestbodyfailed: %w", marshalErr)
			}
			bodyReader = bytes.NewReader(data)
		}

		// createHTTPrequest
		req, createErr := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
		if createErr != nil {
			return fmt.Errorf("failed to create request: %w", createErr)
		}

		// setdefaultrequest header
		req.Header.Set("Content-Type", "application/json")

		// setauthenticateToken
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}

		// set自定义request header
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}

		// sendrequest
		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("requestfailed: %w", doErr)
		}
		defer resp.Body.Close()

		// readrespondbody
		responseBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// inspectHTTPstate码
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
