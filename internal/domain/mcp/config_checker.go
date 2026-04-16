package mcp

import (
	"fmt"
	"net/url"
	"strings"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

// CheckMCPConfig check MCP config and report potential issues
func CheckMCPConfig() {
	log.Info("=== MCP config inspect ===")

	// check global enabled state
	globalEnabled := viper.GetBool("mcp.global.enabled")
	log.Infof("global MCP enabled state: %v", globalEnabled)

	if !globalEnabled {
		log.Info("global MCP already disabled, config inspect complete")
		return
	}

	// check reconnect config
	reconnectInterval := viper.GetInt("mcp.global.reconnect_interval")
	maxAttempts := viper.GetInt("mcp.global.max_reconnect_attempts")
	log.Infof("reconnect config: interval=%d second, maximum retry count=%d", reconnectInterval, maxAttempts)

	// check server config
	var serverConfigs []MCPServerConfig
	if err := viper.UnmarshalKey("mcp.global.servers", &serverConfigs); err != nil {
		log.Errorf("❌ parse MCP server config failed: %v", err)
		return
	}

	if len(serverConfigs) == 0 {
		log.Warn("⚠️  no MCP server configured")
		return
	}

	log.Infof("total configured %d MCP servers:", len(serverConfigs))

	enabledCount := 0
	problemCount := 0

	for i, config := range serverConfigs {
		status := "✅"
		issues := []string{}

		// check name
		if config.Name == "" {
			status = "❌"
			issues = append(issues, "name is empty")
			problemCount++
		}

		transportType, endpoint, err := endpointForConfig(config)
		if err != nil {
			status = "❌"
			issues = append(issues, err.Error())
			problemCount++
		} else {
			if _, parseErr := url.ParseRequestURI(endpoint); parseErr != nil {
				status = "❌"
				issues = append(issues, "URL format not correct")
				problemCount++
			}
			if transportType == "sse" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				status = "⚠️"
				issues = append(issues, "SSE URL format may not be correct")
			}
		}

		// check enabled state
		if config.Enabled {
			enabledCount++
		}

		// output inspect result
		issueStr := ""
		if len(issues) > 0 {
			issueStr = fmt.Sprintf(" - issue: %s", strings.Join(issues, ", "))
		}

		log.Infof("  [%d] %s %s (URL: %s, enabled: %v)%s",
			i+1, status, config.Name, endpointForLog(config), config.Enabled, issueStr)
	}

	// summary
	log.Infof("config inspect complete: %d servers enabled, %d have issues", enabledCount, problemCount)

	if problemCount > 0 {
		log.Warn("⚠️  discovered config issues, please inspect above errors and fix")
	}

	log.Info("=== MCP config inspect complete ===")
}

func endpointForLog(config MCPServerConfig) string {
	_, endpoint, err := endpointForConfig(config)
	if err != nil {
		if strings.TrimSpace(config.Url) != "" {
			return config.Url
		}
		return config.SSEUrl
	}
	return endpoint
}
