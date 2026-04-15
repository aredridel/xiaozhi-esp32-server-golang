package controllers

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebSocketControllerInterface defines the WebSocket controller interface
type WebSocketControllerInterface interface {
	RequestMcpToolDetailsFromClient(ctx context.Context, agentID string) ([]MCPTool, error)
}

// GetAgentMcpToolsCommon common function to get agent MCP tools list
// This function can be used by both admin and regular user controllers
func GetAgentMcpToolsCommon(
	c *gin.Context,
	agentID string,
	webSocketController WebSocketControllerInterface,
	agentValidator func(agentID string) error, // Function to validate agent permissions
) {
	log.Printf("GetAgentMcpToolsCommon starting execution, agentID: %s", agentID)

	if agentID == "" {
		log.Printf("Error: agent_id parameter is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// Validate agent permissions (validation logic provided by caller)
	if err := agentValidator(agentID); err != nil {
		log.Printf("Agent validation failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Agent validation successful, starting WebSocket controller check")

	// Check if WebSocket controller exists
	if webSocketController == nil {
		// When WebSocket controller doesn't exist, return empty list instead of error
		log.Printf("WebSocket controller not initialized, returning empty tools list")
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	log.Printf("WebSocket controller exists, starting MCP tools list request")

	// Create context
	ctx := context.Background()

	// Get tool details (includes schema and examples)
	tools, err := webSocketController.RequestMcpToolDetailsFromClient(ctx, agentID)
	if err != nil {
		log.Printf("Failed to get MCP tools list: %v", err)
		// If get fails, return empty list instead of error
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	log.Printf("Successfully got MCP tools list: count=%d", len(tools))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": tools}})
}
