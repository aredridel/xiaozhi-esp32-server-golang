package http

import "time"

// ClientConfig HTTPclient-sideconfig
type ClientConfig struct {
	BaseURL   string        // foundationURL
	AuthToken string        // authenticateToken（optional）
	Timeout   time.Duration // requesttimeouttime
	MaxRetries int          // maximumretrytimescount（default3times）
}

// RequestOptions requestoption
type RequestOptions struct {
	Method      string                 // HTTPmethod
	Path        string                 // requestpath
	QueryParams map[string]string      // queryparameter
	Headers     map[string]string      // 自定义request header
	Body        interface{}             // requestbody（willautomaticserializeisJSON）
	Response    interface{}             // respondbody（willautomaticdeserialize）
}

