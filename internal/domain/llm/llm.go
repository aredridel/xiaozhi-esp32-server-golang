package llm

import (
	"context"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

// ConvertMCPToolsToEinoTools converts MCP tools to Eino ToolInfo format
func ConvertMCPToolsToEinoTools(ctx context.Context, mcpTools map[string]interface{}) ([]*schema.ToolInfo, error) {
	var einoTools []*schema.ToolInfo

	for toolName, mcpTool := range mcpTools {
		// try to get tool info
		if invokableTool, ok := mcpTool.(interface {
			Info(context.Context) (*schema.ToolInfo, error)
		}); ok {
			toolInfo, err := invokableTool.Info(ctx)
			if err != nil {
				log.Errorf("get tool %s info failed: %v", toolName, err)
				continue
			}
			einoTools = append(einoTools, toolInfo)
		} else {
			log.Warnf("tool %s unsupported Info interface, skip convert", toolName)
		}
	}

	log.Infof("successfully converted %d MCP tools to Eino tools", len(einoTools))
	return einoTools, nil
}
