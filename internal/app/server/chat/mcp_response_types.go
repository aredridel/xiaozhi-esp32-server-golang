package chat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

// MCPResponseType define MCP response type
type MCPResponseType string

const (
	// action class: need execute specific action, usually will terminate subsequent process
	MCPResponseTypeAction MCPResponseType = "action"
	// audio resource class: need execute specific action, usually will terminate subsequent process, also no need return stop
	MCPResponseTypeAudio MCPResponseType = "audio"

	// content class: return info content, allow after continue process
	MCPResponseTypeContent MCPResponseType = "content"
	// error class: process error situation
	MCPResponseTypeError MCPResponseType = "error"
)

// MCPResponseBase all MCP response of foundation structure
type MCPResponseBase struct {
	Type      MCPResponseType `json:"type"`
	Success   bool            `json:"success"`
	Timestamp int64           `json:"timestamp"`
	ToolName  string          `json:"tool_name"`
}

// MCPActionResponse action class respond - used for play music, exit to conversation etc need execute action of scenario
type MCPActionResponse struct {
	MCPResponseBase
	Action   string            `json:"action"`
	Message  string            `json:"message"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`
	// control flag
	FinalAction       bool   `json:"final_action"`
	NoFurtherResponse bool   `json:"no_further_response"`
	SilenceLLM        bool   `json:"silence_llm"`
	UserState         string `json:"user_state"`
	Instruction       string `json:"instruction,omitempty"`
}

// MCPActionResponse action class respond - used for play music, exit to conversation etc need execute action of scenario
type MCPAudioResponse struct {
	MCPResponseBase
	Data      []byte            `json:"data"`
	MusicName string            `json:"music_name"`
	Action    string            `json:"action"`
	Status    string            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	// control flag
	FinalAction bool `json:"final_action"`
}

// MCPContentResponse content class respond - used for get time, query info etc return data of scenario
type MCPContentResponse struct {
	MCPResponseBase
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// MCPErrorResponse error class respond - unified of error process
type MCPErrorResponse struct {
	MCPResponseBase
	Error      string `json:"error"`
	ErrorCode  string `json:"error_code,omitempty"`
	Details    string `json:"details,omitempty"`
	Suggestion string `json:"suggestion,omitempty"` // suggestion for user
}

// MCPResponse unified of MCP respond interface
type MCPResponse interface {
	GetType() MCPResponseType
	GetSuccess() bool
	IsTerminal() bool // whether yes terminating operation
	ToJSON() (string, error)
	GetContent() []mcp_go.Content
	GetAction() string // get action type
}

// implement MCPResponse interface
func (r *MCPActionResponse) GetType() MCPResponseType { return MCPResponseTypeAction }
func (r *MCPActionResponse) GetSuccess() bool         { return r.Success }
func (r *MCPActionResponse) IsTerminal() bool         { return r.FinalAction || r.NoFurtherResponse }
func (r *MCPActionResponse) GetAction() string        { return r.Action }
func (r *MCPActionResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.Message,
		},
	}
}

// is MCPAudioResponse add interface method implement
func (r *MCPAudioResponse) GetType() MCPResponseType { return MCPResponseTypeAudio }
func (r *MCPAudioResponse) GetSuccess() bool         { return r.Success }
func (r *MCPAudioResponse) IsTerminal() bool         { return r.FinalAction }
func (r *MCPAudioResponse) GetAction() string        { return r.Action }
func (r *MCPAudioResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.MusicName,
		},
		mcp_go.AudioContent{
			Type:     "audio",
			Data:     base64.StdEncoding.EncodeToString(r.Data),
			MIMEType: "audio/mpeg",
		},
	}
}

func (r *MCPContentResponse) GetType() MCPResponseType { return MCPResponseTypeContent }
func (r *MCPContentResponse) GetSuccess() bool         { return r.Success }
func (r *MCPContentResponse) IsTerminal() bool         { return false } // content class usually no terminate
func (r *MCPContentResponse) GetAction() string        { return "" }    // content class no action
func (r *MCPContentResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.Message,
		},
	}
}

func (r *MCPErrorResponse) GetType() MCPResponseType { return MCPResponseTypeError }
func (r *MCPErrorResponse) GetSuccess() bool         { return r.Success }
func (r *MCPErrorResponse) IsTerminal() bool         { return false } // error class allow after continue process
func (r *MCPErrorResponse) GetAction() string        { return "" }    // error class no action
func (r *MCPErrorResponse) GetContent() []mcp_go.Content {
	return []mcp_go.Content{
		mcp_go.TextContent{
			Type: "text",
			Text: r.Error,
		},
	}
}

// ToJSON method implement
func (r *MCPActionResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

// is MCPAudioResponse add ToJSON method
func (r *MCPAudioResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

func (r *MCPContentResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

func (r *MCPErrorResponse) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	return string(data), err
}

// convenient construct function

// NewActionResponse create action class respond
func NewActionResponse(toolName, action, message, status string, terminal bool) *MCPActionResponse {
	return &MCPActionResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeAction,
			Success:   true,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Action:            action,
		Message:           message,
		Status:            status,
		FinalAction:       terminal,
		NoFurtherResponse: terminal,
		SilenceLLM:        terminal,
	}
}

// NewAudioResponse create audio class respond - fix positive return type
func NewAudioResponse(toolName, action, status string, terminal bool, data []byte) *MCPAudioResponse {
	return &MCPAudioResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeAudio,
			Success:   true,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Data:        data,
		Action:      action,
		Status:      status,
		FinalAction: terminal,
	}
}

// NewContentResponse create content class respond
func NewContentResponse(toolName string, data interface{}, message string) *MCPContentResponse {
	return &MCPContentResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeContent,
			Success:   true,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Data:    data,
		Message: message,
	}
}

// NewErrorResponse create error class respond
func NewErrorResponse(toolName, error, errorCode, suggestion string) *MCPErrorResponse {
	return &MCPErrorResponse{
		MCPResponseBase: MCPResponseBase{
			Type:      MCPResponseTypeError,
			Success:   false,
			Timestamp: time.Now().Unix(),
			ToolName:  toolName,
		},
		Error:      error,
		ErrorCode:  errorCode,
		Suggestion: suggestion,
	}
}

// ParseMCPResponse from JSON char string parse MCP respond
func ParseMCPResponse(jsonStr string) (MCPResponse, error) {
	var base MCPResponseBase
	if err := json.Unmarshal([]byte(jsonStr), &base); err != nil {
		return nil, err
	}

	switch base.Type {
	case MCPResponseTypeAction:
		var response MCPActionResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	case MCPResponseTypeAudio:
		var response MCPAudioResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	case MCPResponseTypeContent:
		var response MCPContentResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	case MCPResponseTypeError:
		var response MCPErrorResponse
		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return nil, err
		}
		return &response, nil
	default:
		return NewErrorResponse("unknown", "unknown response type", "INVALID_TYPE", "please inspect tool implement"), fmt.Errorf("unknown response type: %s", base.Type)
	}
}
