package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	data_client "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

type toolCallResponseSummary struct {
	invokeToolSuccess bool
	hasMediaOutput    bool
}

// handleToolCallResponse processtoolcallrespond
func (l *LLMManager) handleToolCallResponse(ctx context.Context, respMsg *schema.Message, tools []schema.ToolCall, executor *toolCallExecutor) (toolCallResponseSummary, error) {
	if len(tools) == 0 {
		return toolCallResponseSummary{}, nil
	}

	log.Infof("process %d 个toolcall", len(tools))
	if executor == nil {
		executor = newToolCallExecutor(l, ctx)
		executor.Submit(tools)
	}

	var messageList []*schema.Message

	// onlyhavewhenrespMsghaveinside容（ContentnoisemptyorToolCallsnoisempty）whenonly thenaddtomessageList
	// avoidsaveemptyofassistantmessagecauseaftercontinueLLMcallout现400error
	if respMsg != nil && (respMsg.Content != "" || len(respMsg.ToolCalls) > 0) {
		messageList = append(messageList, respMsg)
	}

	invokeToolSuccess := false
	findExitTool := false
	shouldStopLLMProcessing := false
	hasMediaOutput := false

	results := executor.Wait()
	for _, result := range results {
		if result.invokeToolSuccess {
			invokeToolSuccess = true
		}
		if result.findExitTool {
			findExitTool = true
		}
		if result.shouldStopLLMProcessing {
			shouldStopLLMProcessing = true
		}
		if result.hasMediaOutput {
			hasMediaOutput = true
		}
		messageList = append(messageList, result.message)
	}

	if len(messageList) > 0 {
		for _, msg := range messageList {
			// filter掉Contentisemptyofassistantmessage，avoidsavetohistoryrecordin
			// emptyofassistantmessagewillcauseaftercontinueLLMcallwhenout现400error
			if msg != nil && msg.Role == schema.Assistant && msg.Content == "" && len(msg.ToolCalls) == 0 {
				log.Debugf("skipsaveemptyofassistantmessage")
				continue
			}
			l.AddLlmMessage(ctx, msg)
		}
	}

	if l.clientState.GetMemoryMode() == data_client.MemoryModeNone {
		ctx = appendToolRoundMessagesToContext(ctx, messageList)
	}

	executor.WaitMedia()

	if findExitTool {
		// publishexitchatevent
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: l.clientState,
			Reason:      "toolcallexit",
			TriggerType: "tool_call",
			UserText:    "",
			Timestamp:   time.Now(),
		})

		return toolCallResponseSummary{
			invokeToolSuccess: invokeToolSuccess,
			hasMediaOutput:    hasMediaOutput,
		}, nil
	}

	// iftoolcallsuccessfulandnobemarkisstopprocess，thencontinueLLMcall
	if invokeToolSuccess && !shouldStopLLMProcessing {
		l.DoLLmRequest(ctx, nil, l.einoTools, true, nil)
	}

	return toolCallResponseSummary{
		invokeToolSuccess: invokeToolSuccess,
		hasMediaOutput:    hasMediaOutput,
	}, nil
}

type toolCallExecutionResult struct {
	order                   int
	message                 *schema.Message
	invokeToolSuccess       bool
	findExitTool            bool
	shouldStopLLMProcessing bool
	hasMediaOutput          bool
}

type toolCallExecutor struct {
	manager *LLMManager
	ctx     context.Context
	toolCtx context.Context

	mu             sync.Mutex
	submittedCall  map[string]struct{}
	submitSequence int
	results        []toolCallExecutionResult

	wg      sync.WaitGroup
	mediaWg sync.WaitGroup
}

func newToolCallExecutor(manager *LLMManager, ctx context.Context) *toolCallExecutor {
	toolCtx := ctx
	if chatSessionOperator, ok := ctx.Value("chat_session_operator").(ChatSessionOperator); ok {
		toolCtx = context.WithValue(ctx, "chat_session_operator", chatSessionOperator)
	}

	return &toolCallExecutor{
		manager:       manager,
		ctx:           ctx,
		toolCtx:       toolCtx,
		submittedCall: make(map[string]struct{}),
	}
}

func (e *toolCallExecutor) Submit(toolCalls []schema.ToolCall) {
	for _, tc := range toolCalls {
		callID := tc.ID
		if callID == "" {
			callID = fmt.Sprintf("auto_%s_%s", tc.Function.Name, tc.Function.Arguments)
		}

		e.mu.Lock()
		if _, exists := e.submittedCall[callID]; exists {
			e.mu.Unlock()
			continue
		}
		e.submittedCall[callID] = struct{}{}
		order := e.submitSequence
		e.submitSequence++
		e.wg.Add(1)
		e.mu.Unlock()

		toolCall := tc
		go func() {
			defer e.wg.Done()
			result := e.executeToolCall(order, toolCall)

			e.mu.Lock()
			e.results = append(e.results, result)
			e.mu.Unlock()
		}()
	}
}

func (e *toolCallExecutor) Wait() []toolCallExecutionResult {
	e.wg.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()

	results := append([]toolCallExecutionResult(nil), e.results...)
	sort.Slice(results, func(i, j int) bool {
		return results[i].order < results[j].order
	})
	return results
}

func (e *toolCallExecutor) WaitMedia() {
	e.mediaWg.Wait()
}

func (e *toolCallExecutor) executeToolCall(order int, toolCall schema.ToolCall) toolCallExecutionResult {
	resultMessage := &schema.Message{
		Role:       schema.Tool,
		ToolCallID: toolCall.ID,
	}

	state := e.manager.clientState
	toolName := toolCall.Function.Name
	toolObj, ok := mcp.GetToolByName(state.DeviceID, state.AgentID, toolName, state.DeviceConfig.MCPServiceNames)
	if !ok || toolObj == nil {
		log.Errorf("not找totool: %s", toolName)
		resultMessage.Content = fmt.Sprintf("not找totool: %s", toolName)
		return toolCallExecutionResult{order: order, message: resultMessage}
	}

	log.Infof("performtoolcallrequest: %s, parameter: %+v", toolName, toolCall.Function.Arguments)
	startTs := time.Now().UnixMilli()
	fcResult, err := toolObj.InvokableRun(e.toolCtx, toolCall.Function.Arguments)
	if err != nil {
		log.Errorf("toolcallfailed: %v", err)
		resultMessage.Content = fmt.Sprintf("tool %s callfailed: %v", toolName, err)
		return toolCallExecutionResult{order: order, message: resultMessage}
	}
	costTs := time.Now().UnixMilli() - startTs
	if len(fcResult) > 2048 {
		log.Infof("toolcallresult len: %d, time consumption: %dms", len(fcResult), costTs)
	} else {
		log.Infof("toolcallresult %s, time consumption: %dms", fcResult, costTs)
	}

	execResult := toolCallExecutionResult{
		order:             order,
		message:           resultMessage,
		invokeToolSuccess: true,
	}
	normalizedContent := fcResult

	var contentList []mcp_go.Content
	if mcpResp, ok := e.manager.handleLocalToolResult(fcResult); ok {
		if mcpResp.GetType() == MCPResponseTypeAction && mcpResp.GetAction() == "exit_conversation" {
			execResult.findExitTool = true
		}
		if actionResp, ok := mcpResp.(*MCPActionResponse); ok {
			if actionResp.FinalAction || actionResp.NoFurtherResponse || actionResp.SilenceLLM {
				execResult.shouldStopLLMProcessing = true
			}
		}
		contentList = mcpResp.GetContent()
	} else if toolCallResult, ok := e.manager.handleToolResult(fcResult); ok {
		if toolCallResult.IsError {
			log.Errorf("toolcallfailed: %s, errormark: %t", fcResult, toolCallResult.IsError)
		}
		contentList = toolCallResult.Content
	}

	if len(contentList) > 0 {
		var mcpContent string
		for _, content := range contentList {
			if audioContent, ok := content.(mcp_go.AudioContent); ok {
				log.Debugf("calltool %s returnaudioresourcelength: %d", toolName, len(audioContent.Data))
				mcpContent = "executesuccessful"
				if err := e.manager.handleAudioContent(e.ctx, toolName, audioContent, &e.mediaWg); err != nil {
					log.Errorf("mcpplayaudioresourcefailed: %v", err)
					mcpContent = "executefailed"
				}
				execResult.shouldStopLLMProcessing = true
				execResult.hasMediaOutput = true
				break
			}
			if resourceLink, ok := content.(mcp_go.ResourceLink); ok {
				log.Debugf("calltool %s returnresourcelink: %+v", toolName, resourceLink)
				mcpContent = "executesuccessful"
				if err := e.manager.handleResourceLink(e.ctx, resourceLink, toolObj, &e.mediaWg); err != nil {
					log.Errorf("mcpplayresourcelinkfailed: %v", err)
					mcpContent = "executefailed"
				}
				execResult.shouldStopLLMProcessing = true
				execResult.hasMediaOutput = true
				break
			}
			if textContent, ok := content.(mcp_go.TextContent); ok {
				log.Debugf("calltool %s returntextresourcelength: %s", toolName, textContent.Text)
				mcpContent += textContent.Text
			}
		}
		if mcpContent != "" {
			normalizedContent = mcpContent
		}
	}

	execResult.message.Content = normalizedContent
	return execResult
}

func (l *LLMManager) handleResourceLink(ctx context.Context, resourceLink mcp_go.ResourceLink, toolCall tool.InvokableTool, wg *sync.WaitGroup) error {
	wg.Add(1)

	source, err := buildMediaSourceFromResourceLink(resourceLink, toolCall)
	if err != nil {
		wg.Done()
		return err
	}

	if l.session == nil || l.session.mediaPlayer == nil {
		wg.Done()
		return fmt.Errorf("session media player not initialized")
	}

	handle, err := l.session.mediaPlayer.PlaySourceWithHandle(ctx, source)
	if err != nil {
		wg.Done()
		return err
	}

	go func() {
		defer wg.Done()
		_ = handle.Wait(ctx)
	}()

	return nil
}

func (l *LLMManager) handleAudioContent(ctx context.Context, realMusicName string, audioContent mcp_go.AudioContent, wg *sync.WaitGroup) error {
	wg.Add(1)

	source, err := buildMediaSourceFromAudioContent(realMusicName, audioContent)
	if err != nil {
		wg.Done()
		log.Errorf("decodeaudio datafailed: %v", err)
		return err
	}

	if l.session == nil || l.session.mediaPlayer == nil {
		wg.Done()
		return fmt.Errorf("session media player not initialized")
	}

	handle, err := l.session.mediaPlayer.PlaySourceWithHandle(ctx, source)
	if err != nil {
		wg.Done()
		return err
	}

	go func() {
		defer wg.Done()
		_ = handle.Wait(ctx)
	}()

	return nil
}

func buildMediaSourceFromAudioContent(title string, audioContent mcp_go.AudioContent) (MediaSourceDescriptor, error) {
	rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Data)
	if err != nil {
		return MediaSourceDescriptor{}, fmt.Errorf("decodeaudio datafailed: %v", err)
	}

	title = strings.TrimSpace(title)
	if title == "" || title == "executesuccessful" {
		title = "toolaudio"
	}

	return MediaSourceDescriptor{
		Title:      title,
		MIMEType:   audioContent.MIMEType,
		SourceType: MediaSourceTypeInlineAudio,
		Meta: map[string]string{
			"mime_type": audioContent.MIMEType,
			"source":    string(MediaSourceTypeInlineAudio),
		},
		Inline: &InlineAudioSource{
			Data: rawAudioData,
		},
	}, nil
}

func buildMediaSourceFromResourceLink(resourceLink mcp_go.ResourceLink, toolCall tool.InvokableTool) (MediaSourceDescriptor, error) {
	mcpTool, ok := toolCall.(*mcp.McpTool)
	if !ok || mcpTool == nil {
		return MediaSourceDescriptor{}, fmt.Errorf("resource link playonlysupport MCP remotetool")
	}

	serverName := mcpTool.GetServerName()
	endpointSnapshot := mcp.GetServerEndpointSnapshotByName(serverName)
	directAudioURL := strings.TrimSpace(resourceLink.Description)
	readArgs := make(map[string]any)
	if directAudioURL != "" {
		readArgs["url"] = directAudioURL
	}

	title := strings.TrimSpace(resourceLink.Name)
	if title == "" {
		title = strings.TrimSpace(resourceLink.Description)
	}
	if title == "" {
		title = strings.TrimSpace(resourceLink.URI)
	}

	toolName := ""
	if info, err := mcpTool.Info(context.Background()); err == nil && info != nil {
		toolName = info.Name
	}

	return MediaSourceDescriptor{
		Title:      title,
		MIMEType:   resourceLink.MIMEType,
		SourceType: MediaSourceTypeMCPResource,
		Meta: map[string]string{
			"source":       string(MediaSourceTypeMCPResource),
			"server_name":  serverName,
			"endpoint":     endpointSnapshot,
			"resource_uri": resourceLink.URI,
			"audio_url":    directAudioURL,
			"tool_name":    toolName,
		},
		MCP: &MCPMediaSource{
			ServerName:       serverName,
			EndpointSnapshot: endpointSnapshot,
			ToolName:         toolName,
			ResourceURI:      resourceLink.URI,
			DirectAudioURL:   directAudioURL,
			Description:      resourceLink.Description,
			ReadArgs:         readArgs,
			PageSize:         McpReadResourcePageSize,
			Client:           mcpTool.GetClient(),
		},
	}, nil
}

func (l *LLMManager) handleLocalToolResult(toolResult string) (MCPResponse, bool) {
	// ifyesmcpreturnformat, thenparse
	var response MCPResponse
	var err error
	if response, err = ParseMCPResponse(toolResult); err != nil {
		return nil, false
	}
	return response, true
}

func (l *LLMManager) handleToolResult(toolResultStr string) (mcp_go.CallToolResult, bool) {
	var toolResult mcp_go.CallToolResult
	if err := json.Unmarshal([]byte(toolResultStr), &toolResult); err != nil {
		log.Debugf("Failed to unmarshal tool result: %v", err)
		toolResult.Content = []mcp_go.Content{
			mcp_go.TextContent{
				Type: "text",
				Text: toolResultStr,
			},
		}
		return toolResult, true
	}

	return toolResult, true
}
