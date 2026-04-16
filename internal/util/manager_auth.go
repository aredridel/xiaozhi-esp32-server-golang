package util

import (
	"strings"

	"github.com/spf13/viper"
)

const DefaultManagerAuthToken = "xiaozhi_admin_secret_key"
const DefaultManagerEndpointAuthToken = "xiaozhi_mcp_openclaw_secret_key"

// GetManagerAuthToken gets the internal authentication token used for communication between the main program and the control panel.
// priority:
// 1. manager.auth_token
// 2. default values (both endpoints keep consistent)
func GetManagerAuthToken() string {
	if token := strings.TrimSpace(viper.GetString("manager.auth_token")); token != "" {
		return token
	}
	return DefaultManagerAuthToken
}

// GetManagerEndpointAuthToken gets the JWT sign/verify token for MCP/OpenClaw endpoint.
// priority:
// 1. manager.endpoint_auth_token
// 2. default values (need to keep consistent with control panel)
func GetManagerEndpointAuthToken() string {
	if token := strings.TrimSpace(viper.GetString("manager.endpoint_auth_token")); token != "" {
		return token
	}
	return DefaultManagerEndpointAuthToken
}
