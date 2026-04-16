package controllers

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChatHistoryController struct {
	DB            *gorm.DB
	AudioBasePath string // Audio storage base path
	MaxFileSize   int64  // Maximum file size (10MB)
}

// SaveMessageRequest save message request
type SaveMessageRequest struct {
	MessageID     string                 `json:"message_id" binding:"required"`
	DeviceID      string                 `json:"device_id" binding:"required"`
	AgentID       string                 `json:"agent_id" binding:"required"`
	SessionID     string                 `json:"session_id,omitempty"`
	Role          string                 `json:"role" binding:"required,oneof=user assistant system tool"`
	Content       string                 `json:"content" binding:"required"`
	ToolCallID    string                 `json:"tool_call_id,omitempty"`    // Tool call ID (used by Tool role)
	ToolCallsJSON *string                `json:"tool_calls_json,omitempty"` // Tool calls list JSON (used by Assistant role), nil means NULL
	AudioData     string                 `json:"audio_data,omitempty"`      // base64 encoded
	AudioFormat   string                 `json:"audio_format,omitempty"`    // Audio format (passed by client, backend fixed to use wav)
	AudioDuration int                    `json:"audio_duration,omitempty"`
	AudioSize     int                    `json:"audio_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SaveMessage save message
func (c *ChatHistoryController) SaveMessage(ctx *gin.Context) {
	var req SaveMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify device exists (query using device_name field)
	var device models.Device
	if err := c.DB.Where("device_name = ?", req.DeviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Device does not exist"})
			return
		}
		// Other database errors
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query device: " + err.Error()})
		return
	}

	// If AgentID not provided in request, use device associated AgentID
	agentID := req.AgentID
	if agentID == "" && device.AgentID > 0 {
		agentID = fmt.Sprintf("%d", device.AgentID)
	}

	// If AgentID is still empty, skip saving
	if agentID == "" {
		ctx.JSON(http.StatusOK, gin.H{"message": "Skip saving: no associated AgentID"})
		return
	}

	message := &models.ChatMessage{
		MessageID:     req.MessageID,
		DeviceID:      req.DeviceID,
		AgentID:       agentID,
		UserID:        device.UserID,
		SessionID:     req.SessionID,
		Role:          req.Role,
		Content:       req.Content,
		ToolCallID:    req.ToolCallID,
		ToolCallsJSON: req.ToolCallsJSON,
		Metadata:      req.Metadata,
	}

	// Check if message already exists (avoid duplicate creation)
	var existingMessage models.ChatMessage
	err := c.DB.Where("message_id = ?", req.MessageID).First(&existingMessage).Error
	if err == nil {
		// Message exists, update audio data (if provided)
		if req.AudioData != "" {
			audioPath, err := c.saveAudioFile(req.MessageID, req.AudioData)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio file: " + err.Error()})
				return
			}

			// If there was an audio file before, delete it first
			if existingMessage.AudioPath != "" {
				c.deleteAudioFile(existingMessage.AudioPath)
			}

			// Update message
			updates := map[string]interface{}{
				"audio_path":   audioPath,
				"audio_format": "wav",
			}
			if req.AudioSize > 0 {
				updates["audio_size"] = req.AudioSize
			}
			if req.AudioDuration > 0 {
				updates["audio_duration"] = req.AudioDuration
			}

			// Update metadata (merge)
			if existingMessage.Metadata == nil {
				existingMessage.Metadata = make(map[string]interface{})
			}
			if req.Metadata != nil {
				for k, v := range req.Metadata {
					existingMessage.Metadata[k] = v
				}
			}
			// Manually serialize metadata to MetadataJSON (because Updates won't trigger BeforeSave hook)
			metadataJSONBytes, err := json.Marshal(existingMessage.Metadata)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize metadata: " + err.Error()})
				return
			}
			updates["metadata"] = string(metadataJSONBytes)

			if err := c.DB.Model(&existingMessage).Updates(updates).Error; err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update message"})
				return
			}
			ctx.JSON(http.StatusOK, existingMessage)
			return
		}
		// Message exists and no audio data, return directly
		ctx.JSON(http.StatusOK, existingMessage)
		return
	} else if err != gorm.ErrRecordNotFound {
		// Query error (not "record not found")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query message: " + err.Error()})
		return
	}

	// Message does not exist, create new message
	// Process audio data - save to file system (fixed to wav format, two-level hash distribution)
	if req.AudioData != "" {
		audioPath, err := c.saveAudioFile(req.MessageID, req.AudioData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio file: " + err.Error()})
			return
		}
		message.AudioPath = audioPath
		message.AudioFormat = "wav" // Fixed to wav format
		if req.AudioSize > 0 {
			message.AudioSize = &req.AudioSize
		}
		if req.AudioDuration > 0 {
			message.AudioDuration = &req.AudioDuration
		}
	}

	if err := c.DB.Create(message).Error; err != nil {
		// If database save fails, delete the saved audio file
		if message.AudioPath != "" {
			c.deleteAudioFile(message.AudioPath)
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, message)
}

// GetMessages get message list (grouped by agentId)
func (c *ChatHistoryController) GetMessages(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	agentID := ctx.Query("agent_id")
	deviceID := ctx.Query("device_id")
	sessionID := ctx.Query("session_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "50"))
	role := ctx.Query("role") // user/assistant

	// Build query
	query := c.DB.Model(&models.ChatMessage{}).
		Where("user_id = ? AND is_deleted = ?", userID, false)

	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	query.Count(&total)

	var messages []models.ChatMessage
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      messages,
	})
}

// DeleteMessage delete message (soft delete, immediately delete audio file)
func (c *ChatHistoryController) DeleteMessage(ctx *gin.Context) {
	id := ctx.Param("id")

	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get message information
	var message models.ChatMessage
	if err := c.DB.Where("id = ? AND user_id = ?", id, userID).First(&message).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Message does not exist"})
		return
	}

	// Delete audio file first (if exists)
	if message.AudioPath != "" {
		if err := c.deleteAudioFile(message.AudioPath); err != nil {
			// Log but don't affect delete operation
			log.Printf("Failed to delete audio file: %v", err)
		}
	}

	// Soft delete message
	if err := c.DB.Model(&models.ChatMessage{}).
		Where("id = ?", id).
		Update("is_deleted", true).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// GetMessagesByAgent get message summary by AgentID (supports filtering)
func (c *ChatHistoryController) GetMessagesByAgent(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	agentID := ctx.Param("agent_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "50"))
	role := ctx.Query("role")            // user/assistant
	deviceID := ctx.Query("device_id")   // Device ID filter
	startDate := ctx.Query("start_date") // Start date YYYY-MM-DD
	endDate := ctx.Query("end_date")     // End date YYYY-MM-DD

	// Build query
	query := c.DB.Model(&models.ChatMessage{}).
		Where("user_id = ? AND agent_id = ? AND is_deleted = ?", userID, agentID, false)

	// Role filter
	if role != "" {
		query = query.Where("role = ?", role)
	}

	// Device filter
	if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	// Date range filter
	if startDate != "" {
		if startTime, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", startTime)
		}
	}
	if endDate != "" {
		if endTime, err := time.Parse("2006-01-02", endDate); err == nil {
			// End date includes the whole day
			endTime = endTime.Add(24 * time.Hour)
			query = query.Where("created_at < ?", endTime)
		}
	}

	// Calculate total count
	var total int64
	query.Count(&total)

	// Paginated query (descending by time, newest first, frontend will reverse array to put newest at bottom)
	var messages []models.ChatMessage
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      messages,
	})
}

// ExportMessages export chat history (JSON format)
func (c *ChatHistoryController) ExportMessages(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	agentID := ctx.Query("agent_id")
	deviceID := ctx.Query("device_id")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	// Build query
	query := c.DB.Model(&models.ChatMessage{}).
		Where("user_id = ? AND is_deleted = ?", userID, false)

	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if startDate != "" {
		if startTime, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", startTime)
		}
	}
	if endDate != "" {
		if endTime, err := time.Parse("2006-01-02", endDate); err == nil {
			// End date includes the whole day
			endTime = endTime.Add(24 * time.Hour)
			query = query.Where("created_at < ?", endTime)
		}
	}

	var messages []models.ChatMessage
	if err := query.Order("created_at ASC").Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Export failed"})
		return
	}

	// Set response headers for download
	ctx.Header("Content-Type", "application/json")
	ctx.Header("Content-Disposition", "attachment; filename=chat_history_"+time.Now().Format("20060102_150405")+".json")
	ctx.JSON(http.StatusOK, gin.H{
		"export_time": time.Now().Format("2006-01-02 15:04:05"),
		"total":       len(messages),
		"messages":    messages,
	})
}

// saveAudioFile save audio file to file system (two-level hash distribution)
func (c *ChatHistoryController) saveAudioFile(messageID, audioDataBase64 string) (string, error) {
	// Decode base64 audio data
	audioData, err := base64.StdEncoding.DecodeString(audioDataBase64)
	if err != nil {
		return "", fmt.Errorf("Failed to decode audio data: %v", err)
	}

	// Check file size
	if int64(len(audioData)) > c.MaxFileSize {
		return "", fmt.Errorf("Audio file size exceeds limit: %d > %d", len(audioData), c.MaxFileSize)
	}

	// Calculate MD5 of message_id as filename (excluding suffix)
	fileNameHash := fmt.Sprintf("%x", md5.Sum([]byte(messageID)))

	// Calculate two-level hash for directory distribution
	hash1 := fileNameHash[0:2] // First 2 characters
	hash2 := fileNameHash[2:4] // Characters 3-4

	// Build file path: {base_path}/{hash1}/{hash2}/{md5(message_id)}.wav
	relativePath := fmt.Sprintf("%s/%s/%s.wav", hash1, hash2, fileNameHash)
	fullPath := filepath.Join(c.AudioBasePath, relativePath)

	// Create directory
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("Failed to create directory: %v", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, audioData, 0644); err != nil {
		return "", fmt.Errorf("Failed to write file: %v", err)
	}

	// Return relative path (for database storage)
	return relativePath, nil
}

// deleteAudioFile delete audio file
func (c *ChatHistoryController) deleteAudioFile(relativePath string) error {
	fullPath := filepath.Join(c.AudioBasePath, relativePath)
	return os.Remove(fullPath)
}

// GetAudioFile get audio file (forwarded via Golang)
func (c *ChatHistoryController) GetAudioFile(ctx *gin.Context) {
	id := ctx.Param("id")

	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get message information
	var message models.ChatMessage
	if err := c.DB.Where("id = ? AND user_id = ? AND is_deleted = ?", id, userID, false).First(&message).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Message does not exist"})
		return
	}

	if message.AudioPath == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Audio file does not exist"})
		return
	}

	// Read audio file
	fullPath := filepath.Join(c.AudioBasePath, message.AudioPath)
	audioData, err := os.ReadFile(fullPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read audio file"})
		return
	}

	// Set response headers (wav format)
	ctx.Header("Content-Type", "audio/wav")
	ctx.Header("Content-Length", strconv.Itoa(len(audioData)))
	ctx.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", filepath.Base(message.AudioPath)))

	// Forward audio data
	ctx.Data(http.StatusOK, "audio/wav", audioData)
}

// GetMessagesForInit get message list (for initialization loading, internal service interface, no auth required)
func (c *ChatHistoryController) GetMessagesForInit(ctx *gin.Context) {
	deviceID := ctx.Query("device_id")
	agentID := ctx.Query("agent_id")
	sessionID := ctx.Query("session_id")
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	if deviceID == "" || agentID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "device_id and agent_id cannot be empty"})
		return
	}

	// Build query (don't filter by user_id, as this is an internal service interface)
	query := c.DB.Model(&models.ChatMessage{}).
		Where("device_id = ? AND agent_id = ? AND is_deleted = ?", deviceID, agentID, false)

	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}

	var messages []models.ChatMessage
	// First get latest N items, then reverse to chronological order (old -> new) for LLM use
	if err := query.Order("created_at DESC").
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}

	// Reverse to ensure return order is old -> new
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Convert to response format (text only, no audio)
	messageItems := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		item := map[string]interface{}{
			"message_id": msg.MessageID,
			"role":       msg.Role,
			"content":    msg.Content,
			"created_at": msg.CreatedAt.Format(time.RFC3339),
		}
		// Return tool_call_id directly (if exists)
		if msg.ToolCallID != "" {
			item["tool_call_id"] = msg.ToolCallID
		}
		// Return tool_calls directly (if exists)
		if msg.ToolCallsJSON != nil && *msg.ToolCallsJSON != "" {
			var toolCalls []interface{}
			if err := json.Unmarshal([]byte(*msg.ToolCallsJSON), &toolCalls); err == nil {
				item["tool_calls"] = toolCalls
			}
		}
		messageItems = append(messageItems, item)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"messages": messageItems,
	})
}

// UpdateMessageAudioRequest update message audio request
type UpdateMessageAudioRequest struct {
	AudioData   string                 `json:"audio_data" binding:"required"`
	AudioFormat string                 `json:"audio_format"`
	AudioSize   int                    `json:"audio_size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageAudio update message audio
func (c *ChatHistoryController) UpdateMessageAudio(ctx *gin.Context) {
	messageID := ctx.Param("message_id")
	if messageID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "message_id cannot be empty"})
		return
	}

	var req UpdateMessageAudioRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find message
	var message models.ChatMessage
	if err := c.DB.Where("message_id = ?", messageID).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Message does not exist, skip update (may have been skipped during SaveMessage due to no AgentID)
			ctx.JSON(http.StatusOK, gin.H{"message": "Skip update: message does not exist"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query message"})
		return
	}

	// If message has no associated AgentID, skip update
	if message.AgentID == "" {
		ctx.JSON(http.StatusOK, gin.H{"message": "Skip update: no associated AgentID"})
		return
	}

	// Save audio file
	if req.AudioData != "" {
		audioPath, err := c.saveAudioFile(messageID, req.AudioData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio file: " + err.Error()})
			return
		}

		// If there was an audio file before, delete it first
		if message.AudioPath != "" {
			c.deleteAudioFile(message.AudioPath)
		}

		// Update message
		updates := map[string]interface{}{
			"audio_path":   audioPath,
			"audio_format": "wav",
		}
		if req.AudioSize > 0 {
			updates["audio_size"] = req.AudioSize
		}

		// Update metadata
		if message.Metadata == nil {
			message.Metadata = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			message.Metadata[k] = v
		}
		// Manually serialize metadata to MetadataJSON (because Updates won't trigger BeforeSave hook)
		metadataJSONBytes, err := json.Marshal(message.Metadata)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize metadata: " + err.Error()})
			return
		}
		updates["metadata"] = string(metadataJSONBytes)

		if err := c.DB.Model(&message).Updates(updates).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update message"})
			return
		}
	}

	ctx.JSON(http.StatusOK, message)
}
