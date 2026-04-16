package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp_manager "xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"

	//"github.com/scroot/music-sd/pkg/netease"
	//"github.com/scroot/music-sd/pkg/qq"
	"github.com/spf13/viper"
)

type LocalMcpTool struct {
	Name        string
	Description string
	Params      any
	Handle      mcp_manager.LocalToolHandler
}

// InitChatLocalMCPTools initialize chat-related local MCP tools
func InitChatLocalMCPTools() {
	manager := mcp_manager.GetLocalMCPManager()

	log.Info("initialize chat-related local MCP tools...")

	localTools := map[string]LocalMcpTool{
		/*"get_current_datetime": {
			Name:        "get_current_datetime",
			Description: "getcurrent time and date information",
			Params:      struct{}{},
			Handle:      getCurrentDateTimeHandler,
		},*/
		"exit_conversation": {
			Name:        "exit_conversation",
			Description: "used when user explicitly indicates to end conversation, exit system, or say goodbye, used to gracefully close current chat session",
			Params:      struct{}{},
			Handle:      exitConversationHandler,
		},
		"clear_conversation_history": {
			Name:        "clear_conversation_history",
			Description: "used when user requests to clear, delete, or reset conversation history, used to clear all conversation history of current session",
			Params:      struct{}{},
			Handle:      clearConversationHistoryHandler,
		},
		"switch_device_role": {
			Name:        "switch_device_role",
			Description: "used when user requests to switch current device to a certain role, parameter role_name supports fuzzy matching (will match in global roles and user roles belonging to this device)",
			Params:      SwitchDeviceRoleParams{},
			Handle:      switchDeviceRoleHandler,
		},
		"restore_device_default_role": {
			Name:        "restore_device_default_role",
			Description: "used when user requests to restore device default role or cancel current device role override",
			Params:      struct{}{},
			Handle:      restoreDeviceDefaultRoleHandler,
		},
		"search_knowledge": {
			Name:        "search_knowledge",
			Description: "when user questions require factual basis, process rules, parameter details, or document clauses, retrieve current agent associated knowledge base and return relevant snippets; optionally pass knowledge_base_ids to only search specified knowledge libraries; do not call for casual chat or pure creation scenarios",
			Params:      SearchKnowledgeParams{},
			Handle:      searchKnowledgeHandler,
		},
		/*"play_music": {
			Name:        "play_music",
			Description: "when user wants to listen to music, when bored, when want to play to empty brain use, used for playing specified name of music, when user wants random listen to first music please recommend out concrete song name, when have multiple music play tools priority use this tool, **this tool call time consumption relatively long, need first return friendly transition hint phrase**",
			Params:      PlayMusicParams{},
			Handle:      playMusicHandler,
		},*/
	}

	for toolName, localTool := range localTools {
		// only skip when config explicitly set to false, enable when config not exist or true
		if viper.IsSet("local_mcp."+toolName) && !viper.GetBool("local_mcp."+toolName) {
			continue
		}
		err := manager.RegisterToolFunc(
			localTool.Name,
			localTool.Description,
			localTool.Params,
			localTool.Handle,
		)
		if err != nil {
			log.Errorf("register local MCP tool %s failed: %+v", toolName, err)
		}
	}

	log.Info("chat-related local MCP tools initialization completed")
}

func RegisterLocalMcpFunc(name string, description string, params any, handle mcp_manager.LocalToolHandler) error {
	manager := mcp_manager.GetLocalMCPManager()

	err := manager.RegisterToolFunc(
		name,
		description,
		params,
		handle,
	)
	if err != nil {
		log.Errorf("register local MCP tool %s failed: %+v", name, err)
		return err
	}
	return nil
}

type SwitchDeviceRoleParams struct {
	RoleName string `json:"role_name" description:"target role name, supports fuzzy matching" required:"true"`
}

type SearchKnowledgeParams struct {
	Query            string `json:"query" description:"query content to search" required:"true"`
	TopK             int    `json:"top_k,omitempty" description:"return count, default 5"`
	KnowledgeBaseIDs []uint `json:"knowledge_base_ids,omitempty" description:"optional: only search within these knowledge library IDs (current agent already related)"`
}

// playMusicHandler play music processing function
func playMusicHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute play music tool")

	// parse parameters
	var params PlayMusicParams

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse("play_music", "parameter parsing failed", "PARSE_ERROR", "please check if parameter format is correct")
			return response.ToJSON()
		}
	}

	log.Infof("found ChatSessionOperator, calling LocalMcpPlayMusic method to play music: %s", params.Name)
	audioData, realMusicName, err := GetMusicAudioData(ctx, &params)
	if err != nil {
		log.Errorf("failed to get music data: %v", err)
		response := NewErrorResponse("play_music", fmt.Sprintf("failed to get music data: %v", err), "PLAYBACK_ERROR", "please check music name or network connection")
		return response.ToJSON()
	} else {
		// successful playback - action class response, terminate subsequent process
		response := NewAudioResponse("play_music", "play_music", fmt.Sprintf("start playing music: %s", realMusicName), true, audioData)
		response.MusicName = realMusicName
		return response.ToJSON()
	}

}

/*
// getCurrentDateTimeHandler get current time and date processing function
func getCurrentDateTimeHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute get current time date tool")

	// parse parameters
	var params map[string]interface{}
	timezone := "Local" // default timezone

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if tz, ok := params["timezone"].(string); ok && tz != "" {
				timezone = tz
			}
		}
	}

	now := time.Now()

	// try to parse specified timezone
	if timezone != "Local" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			now = now.In(loc)
		} else {
			log.Warnf("cannot load timezone %s, use local timezone", timezone)
		}
	}

	// construct return data
	data := map[string]interface{}{
		"datetime": map[string]interface{}{
			"formatted":     now.Format("2006-01-02 15:04:05"),
			"iso8601":       now.Format(time.RFC3339),
			"chinese":       formatChineseDateTime(now),
			"unix":          now.Unix(),
			"year":          now.Year(),
			"month":         int(now.Month()),
			"day":           now.Day(),
			"hour":          now.Hour(),
			"minute":        now.Minute(),
			"second":        now.Second(),
			"weekday":       now.Weekday().String(),
			"weekday_zh":    getWeekdayChinese(now.Weekday()),
			"week_number":   getWeekNumber(now),
			"timezone":      timezone,
			"timezone_name": now.Location().String(),
		},
	}

	// create content class response
	response := NewContentResponse("get_current_datetime", data, fmt.Sprintf("current time:%s", formatChineseDateTime(now)))
	// response.Format = "datetime"
	// response.DisplayHint = "can be used to display current date time info"

	log.Infof("get current time date successful: %s", now.Format("2006-01-02 15:04:05"))
	return response.ToJSON(),nil
}
*/
// exitConversationHandler exit conversation processing function
func exitConversationHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute exit conversation tool")

	// parse parameters
	var params map[string]interface{}
	reason := "user actively exited" // default reason

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if r, ok := params["reason"].(string); ok && r != "" {
				reason = r
			}
		}
	}

	// create action class response - terminating operation
	response := NewActionResponse("exit_conversation", "exit_conversation", "conversation is about to end, thank you for using!", "exiting", true)
	response.UserState = "conversation_ended"
	response.Instruction = "conversation has ended, please do not generate additional text responses"
	response.Metadata = map[string]string{
		"reason":           reason,
		"exit_code":        "0",
		"farewell_chinese": "goodbye! looking forward to communicating with you next time.",
		"farewell_english": "Goodbye! Looking forward to our next conversation.",
	}

	log.Infof("exit conversation process complete, reason: %s", reason)

	// get from context ChatSessionOperator and call Close method
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Info("found ChatSessionOperator, calling Close method to close session")
			defer chatSessionOperator.LocalMcpCloseChat()
		} else {
			log.Warn("chat_session_operator obtained from context is not of type ChatSessionOperator")
		}
	} else {
		log.Warn("chat_session_operator not found in context")
	}

	responseStr, err := response.ToJSON()
	if err != nil {
		return "", err
	}

	return responseStr, nil
}

// clearConversationHistoryHandler clear history conversation processing function
func clearConversationHistoryHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute clear conversation history tool")

	// parse parameters
	var params map[string]interface{}
	reason := "user actively cleared history" // default reason

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if r, ok := params["reason"].(string); ok && r != "" {
				reason = r
			}
		}
	}

	// get from context ChatSessionOperator and call LocalMcpClearHistory method
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Info("found ChatSessionOperator, calling LocalMcpClearHistory method to clear history")
			if err := chatSessionOperator.LocalMcpClearHistory(); err != nil {
				log.Errorf("clear history conversation failed: %v", err)
				return "", err
			} else {
				// successful clear - action class response, but not terminate conversation
				response := NewActionResponse("clear_conversation_history", "clear_history", "conversation history cleared successfully, you can start a fresh conversation.", "completed", false)
				response.Metadata = map[string]string{
					"reason": reason,
					"status": "cleared",
				}
				log.Info("history conversation clear successful")

				return response.ToJSON()
			}
		} else {
			log.Warn("chat_session_operator obtained from context is not of type ChatSessionOperator")
			return "", fmt.Errorf("chat_session_operator obtained from context is not of type ChatSessionOperator")
		}
	}
	log.Warn("chat_session_operator not found in context")
	return "", fmt.Errorf("chat_session_operator not found in context")
}

// switchDeviceRoleHandler switch device role processing function
func switchDeviceRoleHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute switch device role tool")

	var params SwitchDeviceRoleParams
	if argumentsInJSON == "" {
		response := NewErrorResponse("switch_device_role", "missing parameter role_name", "MISSING_ROLE_NAME", "please provide the role name to switch to")
		return response.ToJSON()
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		response := NewErrorResponse("switch_device_role", "parameter parsing failed", "PARSE_ERROR", "please inspect role_name parameter format")
		return response.ToJSON()
	}
	params.RoleName = strings.TrimSpace(params.RoleName)
	if params.RoleName == "" {
		response := NewErrorResponse("switch_device_role", "role name cannot be empty", "INVALID_ROLE_NAME", "please provide valid role_name")
		return response.ToJSON()
	}

	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			matchedRoleName, err := chatSessionOperator.LocalMcpSwitchDeviceRole(ctx, params.RoleName)
			if err != nil {
				log.Errorf("switch device role failed: %v", err)
				response := NewErrorResponse("switch_device_role", fmt.Sprintf("failed to switch role: %v", err), "SWITCH_ROLE_FAILED", "please try changing the role name or retry later")
				return response.ToJSON()
			}

			response := NewActionResponse(
				"switch_device_role",
				"switch_device_role",
				fmt.Sprintf("switched to role:%s", matchedRoleName),
				"completed",
				false,
			)
			response.Metadata = map[string]string{
				"requested_role_name": params.RoleName,
				"matched_role_name":   matchedRoleName,
			}
			return response.ToJSON()
		}
		return "", fmt.Errorf("chat_session_operator obtained from context is not of type ChatSessionOperator")
	}

	return "", fmt.Errorf("chat_session_operator not found in context")
}

// restoreDeviceDefaultRoleHandler restore device default role processing function
func restoreDeviceDefaultRoleHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute restore device default role tool")

	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			if err := chatSessionOperator.LocalMcpRestoreDeviceDefaultRole(ctx); err != nil {
				log.Errorf("restore device default role failed: %v", err)
				response := NewErrorResponse("restore_device_default_role", fmt.Sprintf("failed to restore default role: %v", err), "RESTORE_ROLE_FAILED", "please retry later")
				return response.ToJSON()
			}

			response := NewActionResponse(
				"restore_device_default_role",
				"restore_device_default_role",
				"restored device default role",
				"completed",
				false,
			)
			return response.ToJSON()
		}
		return "", fmt.Errorf("chat_session_operator obtained from context is not of type ChatSessionOperator")
	}

	return "", fmt.Errorf("chat_session_operator not found in context")
}

func searchKnowledgeHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("execute knowledge base search tool")

	var params SearchKnowledgeParams
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse("search_knowledge", "parameter parsing failed", "PARSE_ERROR", "please inspect query parameter format")
			return response.ToJSON()
		}
	}
	params.Query = strings.TrimSpace(params.Query)
	if params.Query == "" {
		response := NewErrorResponse("search_knowledge", "query cannot be empty", "INVALID_QUERY", "please provide the content to search")
		return response.ToJSON()
	}
	if params.TopK <= 0 {
		params.TopK = 5
	}

	chatSessionOperatorValue := ctx.Value("chat_session_operator")
	if chatSessionOperatorValue == nil {
		return "", fmt.Errorf("chat_session_operator not found in context")
	}
	chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator)
	if !ok {
		return "", fmt.Errorf("chat_session_operator obtained from context is not of type ChatSessionOperator")
	}

	hits, err := chatSessionOperator.LocalMcpSearchKnowledge(ctx, params.Query, params.TopK, params.KnowledgeBaseIDs)
	if err != nil {
		response := NewErrorResponse("search_knowledge", fmt.Sprintf("information retrieval failed: %v", err), "SEARCH_FAILED", "please retry later")
		return response.ToJSON()
	}

	data := map[string]interface{}{
		"query": params.Query,
		"hits":  hits,
		"count": len(hits),
	}
	if len(hits) == 0 {
		response := NewContentResponse("search_knowledge", data, "not enough relevant information found")
		return response.ToJSON()
	}

	var builder strings.Builder
	for i, hit := range hits {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			continue
		}
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
	}
	msg := strings.TrimSpace(builder.String())
	if msg == "" {
		msg = "relevant information obtained"
	}
	response := NewContentResponse("search_knowledge", data, msg)
	return response.ToJSON()
}

// getWeekNumber get week number
func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

// formatChineseDateTime format Chinese date time
func formatChineseDateTime(t time.Time) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "Sunday",
		time.Monday:    "Monday",
		time.Tuesday:   "Tuesday",
		time.Wednesday: "Wednesday",
		time.Thursday:  "Thursday",
		time.Friday:    "Friday",
		time.Saturday:  "Saturday",
	}

	return fmt.Sprintf("%d year %d month %d day %s %02d:%02d:%02d",
		t.Year(), int(t.Month()), t.Day(),
		weekdays[t.Weekday()],
		t.Hour(), t.Minute(), t.Second(),
	)
}

// getWeekdayChinese get Chinese weekday
func getWeekdayChinese(weekday time.Weekday) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "Sunday",
		time.Monday:    "Monday",
		time.Tuesday:   "Tuesday",
		time.Wednesday: "Wednesday",
		time.Thursday:  "Thursday",
		time.Friday:    "Friday",
		time.Saturday:  "Saturday",
	}
	return weekdays[weekday]
}

// RegisterChatMCPTools public function, for external call register chat MCP tools
func RegisterChatMCPTools() {
	InitChatLocalMCPTools()
}

// play music
func GetMusicAudioData(ctx context.Context, musicParams *PlayMusicParams) ([]byte, string, error) {
	musicName := musicParams.Name
	//welcome := musicParams.Welcome
	welcome := ""
	log.Infof("searching for music: %s , welcome: %s", musicName, welcome)
	// here can get music URL based on music name
	// currently simplified implement, assume musicName is URL or get from config
	musicURL, realMusicName, ierr := getMusicURL(musicName)
	if ierr != nil {
		log.Errorf("failed to get music URL: %v", ierr)
		return nil, "", fmt.Errorf("failed to get music URL: %v", ierr)
	}

	log.Infof("music search successful URL: %s, music name: %s", musicURL, realMusicName)

	client := getHTTPClient()
	req, err := http.NewRequest("GET", musicURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %v", err)
	}

	log.Infof("get music %s data successful, audio data length: %d", realMusicName, len(audioData))

	return audioData, realMusicName, nil
}

/*
func GetMusicAudioData(ctx context.Context, musicParams *PlayMusicParams) ([]byte, string, error) {
	musicName := musicParams.Name
	//welcome := musicParams.Welcome
	welcome := ""
	log.Infof("searching for music: %s , welcome: %s", musicName, welcome)
	// here can get music URL based on music name
	// currently simplified implement, assume musicName is URL or get from config
	musicList := netease.Search(musicName)
	musicList = append(musicList, qq.Search(musicName)...)
	for id, music := range musicList {
		log.Infof("[%2d] %7s | %s %5sMB - %s - %s - %s\n", id, music.Source, music.Duration, music.Size, music.Title, music.Singer, music.Album)
	}

	if len(musicList) <= 0 {
		return nil, "", fmt.Errorf("no music found")
	}
	m := musicList[0]
	m.ParseMusic()
	rc, err := m.ReadCloser()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get music data: %v", err)
	}
	defer rc.Close()

	audioData, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %v", err)
	}

	log.Infof("get music %s data successful, audio data length: %d", m.Name, len(audioData))

	return audioData, m.Name, nil

}
*/
