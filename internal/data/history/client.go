package history

import (
	"context"
	"fmt"
	"time"

	"xiaozhi-esp32-server-golang/internal/components/http"

	"github.com/cloudwego/eino/schema"
)

// MessageType messagetype
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeTool      MessageType = "tool"   // toolcallresult
	MessageTypeSystem    MessageType = "system" // systemmessage（ifuse）
)

// HistoryClientConfig client-sideconfig
type HistoryClientConfig struct {
	BaseURL   string        // Managerafterendpointaddress
	AuthToken string        // authenticateToken
	Timeout   time.Duration // requesttimeout
	Enabled   bool          // whether启use
}

// HistoryClient chat historyHTTPclient-side
type HistoryClient struct {
	client  *http.ManagerClient
	enabled bool
}

// NewHistoryClient createchat historyclient-side
func NewHistoryClient(cfg HistoryClientConfig) *HistoryClient {
	managerClient := http.NewManagerClient(http.ManagerClientConfig{
		BaseURL:    cfg.BaseURL,
		AuthToken:  cfg.AuthToken,
		Timeout:    cfg.Timeout,
		MaxRetries: 3, // defaultretry3times
	})

	return &HistoryClient{
		client:  managerClient,
		enabled: cfg.Enabled,
	}
}

// SaveMessageRequest savemessagerequest
type SaveMessageRequest struct {
	MessageID     string                 `json:"message_id"`
	DeviceID      string                 `json:"device_id"`
	AgentID       string                 `json:"agent_id"`
	SessionID     string                 `json:"session_id,omitempty"`
	Role          MessageType            `json:"role"`
	Content       string                 `json:"content"`
	ToolCallID    string                 `json:"tool_call_id,omitempty"`    // toolcallID（Toolroleuse）
	ToolCallsJSON *string                `json:"tool_calls_json,omitempty"` // toolcalllistJSON（Assistantroleuse），nil indicate NULL
	AudioData     string                 `json:"audio_data,omitempty"`      // base64encode
	AudioFormat   string                 `json:"audio_format,omitempty"`
	AudioDuration int                    `json:"audio_duration,omitempty"`
	AudioSize     int                    `json:"audio_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SaveMessage savemessage
func (c *HistoryClient) SaveMessage(ctx context.Context, req *SaveMessageRequest) error {
	if !c.enabled {
		return nil
	}
	return c.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   "/api/internal/history/messages",
		Body:   req,
	})
}

// UpdateMessageAudioRequest updatemessageaudiorequest
type UpdateMessageAudioRequest struct {
	MessageID   string                 `json:"message_id"`
	AudioData   string                 `json:"audio_data"` // base64encode
	AudioFormat string                 `json:"audio_format"`
	AudioSize   int                    `json:"audio_size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageAudio updatemessageaudio
func (c *HistoryClient) UpdateMessageAudio(ctx context.Context, req *UpdateMessageAudioRequest) error {
	if !c.enabled {
		return nil
	}
	return c.client.DoRequest(ctx, http.RequestOptions{
		Method: "PUT",
		Path:   "/api/internal/history/messages/" + req.MessageID + "/audio",
		Body:   req,
	})
}

// GetMessagesRequest getmessagerequest
type GetMessagesRequest struct {
	DeviceID  string `json:"device_id"`
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id,omitempty"`
	Limit     int    `json:"limit"` // limitcount
}

// GetMessagesResponse getmessagerespond
type GetMessagesResponse struct {
	Messages []MessageItem `json:"messages"`
}

// MessageItem message项（used forinitializeload，noincludeaudio）
type MessageItem struct {
	MessageID  string            `json:"message_id"`
	Role       string            `json:"role"` // user/assistant/tool/system
	Content    string            `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"` // Tool roleuse
	ToolCalls  []schema.ToolCall `json:"tool_calls,omitempty"`   // Assistant roleuse
	CreatedAt  string            `json:"created_at"`
}

// GetMessages from Manager datalibrarygetmessage（used forinitializeload）
func (c *HistoryClient) GetMessages(ctx context.Context, req *GetMessagesRequest) (*GetMessagesResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("history client is disabled")
	}

	// buildqueryparameter
	queryParams := map[string]string{
		"device_id": req.DeviceID,
		"agent_id":  req.AgentID,
		"limit":     fmt.Sprintf("%d", req.Limit),
	}
	if req.SessionID != "" {
		queryParams["session_id"] = req.SessionID
	}

	var resp GetMessagesResponse
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:      "GET",
		Path:        "/api/internal/history/messages",
		QueryParams: queryParams,
		Response:    &resp,
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
