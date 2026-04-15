package mcp

import (
	"fmt"
	"net/url"
	"strings"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

// CheckMCPConfig inspectMCPconfigandreport潜at问题
func CheckMCPConfig() {
	log.Info("=== MCPconfiginspect ===")

	// inspectglobal启usestate
	globalEnabled := viper.GetBool("mcp.global.enabled")
	log.Infof("globalMCP启usestate: %v", globalEnabled)

	if !globalEnabled {
		log.Info("globalMCPalready禁use，configinspectcomplete")
		return
	}

	// inspectreconnectconfig
	reconnectInterval := viper.GetInt("mcp.global.reconnect_interval")
	maxAttempts := viper.GetInt("mcp.global.max_reconnect_attempts")
	log.Infof("reconnectconfig: interval=%dsecond, maximumtrytimescount=%d", reconnectInterval, maxAttempts)

	// inspectserverconfig
	var serverConfigs []MCPServerConfig
	if err := viper.UnmarshalKey("mcp.global.servers", &serverConfigs); err != nil {
		log.Errorf("❌ parseMCPserverconfigfailed: %v", err)
		return
	}

	if len(serverConfigs) == 0 {
		log.Warn("⚠️  notconfig任何MCPserver")
		return
	}

	log.Infof("totalconfig %d 个MCPserver:", len(serverConfigs))

	enabledCount := 0
	problemCount := 0

	for i, config := range serverConfigs {
		status := "✅"
		issues := []string{}

		// inspectname
		if config.Name == "" {
			status = "❌"
			issues = append(issues, "nameisempty")
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
				issues = append(issues, "URLformatnopositive确")
				problemCount++
			}
			if transportType == "sse" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				status = "⚠️"
				issues = append(issues, "SSE URLformatmaynopositive确")
			}
		}

		// inspect启usestate
		if config.Enabled {
			enabledCount++
		}

		// outputinspectresult
		issueStr := ""
		if len(issues) > 0 {
			issueStr = fmt.Sprintf(" - 问题: %s", strings.Join(issues, ", "))
		}

		log.Infof("  [%d] %s %s (URL: %s, 启use: %v)%s",
			i+1, status, config.Name, endpointForLog(config), config.Enabled, issueStr)
	}

	// 总结
	log.Infof("configinspectcomplete: %d个serveralready启use, %d个存at问题", enabledCount, problemCount)

	if problemCount > 0 {
		log.Warn("⚠️  discoverconfig问题，pleaseinspectup述errorand修复")
	}

	log.Info("=== MCPconfiginspectcomplete ===")
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
