package util

import (
	"strings"

	"github.com/spf13/viper"
)

const DefaultManagerAuthToken = "xiaozhi_admin_secret_key"
const DefaultManagerEndpointAuthToken = "xiaozhi_mcp_openclaw_secret_key"

// GetManagerAuthToken getmainprogramandcontrol台之间通useofinternalcall鉴权 Token。
// priority：
// 1. manager.auth_token
// 2. default values（两endpointkeepconsistent）
func GetManagerAuthToken() string {
	if token := strings.TrimSpace(viper.GetString("manager.auth_token")); token != "" {
		return token
	}
	return DefaultManagerAuthToken
}

// GetManagerEndpointAuthToken get MCP/OpenClaw endpointpoint JWT ofsign/verify Token。
// priority：
// 1. manager.endpoint_auth_token
// 2. default values（needandcontrol台keepconsistent）
func GetManagerEndpointAuthToken() string {
	if token := strings.TrimSpace(viper.GetString("manager.endpoint_auth_token")); token != "" {
		return token
	}
	return DefaultManagerEndpointAuthToken
}
