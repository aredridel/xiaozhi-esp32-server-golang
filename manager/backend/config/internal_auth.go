package config

import "strings"

const DefaultInternalAuthToken = "xiaozhi_admin_secret_key"
const DefaultEndpointAuthToken = "xiaozhi_mcp_openclaw_secret_key"

// ResolveInternalAuthToken resolves the console internal service common token.
// Priority:
// 1. internal_auth_token in configuration file
// 2. Default value (consistent with main program)
func ResolveInternalAuthToken(cfg *Config) string {
	if cfg != nil {
		if token := strings.TrimSpace(cfg.InternalAuthToken); token != "" {
			return token
		}
	}
	return DefaultInternalAuthToken
}

// ResolveEndpointAuthToken resolves the MCP/OpenClaw endpoint JWT signing token.
// Priority:
// 1. endpoint_auth_token in configuration file
// 2. Default value (consistent with main program)
func ResolveEndpointAuthToken(cfg *Config) string {
	if cfg != nil {
		if token := strings.TrimSpace(cfg.EndpointAuthToken); token != "" {
			return token
		}
	}
	return DefaultEndpointAuthToken
}
