package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"
	"xiaozhi/manager/backend/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SpeakerGroupController speaker group controller
type SpeakerGroupController struct {
	DB            *gorm.DB
	ServiceURL    string
	HTTPClient    *http.Client
	AudioStorage  *storage.AudioStorage
	HistoryConfig *config.HistoryConfig // Chat history config
}

// NewSpeakerGroupController creates speaker group controller
func NewSpeakerGroupController(db *gorm.DB, cfg *config.Config) *SpeakerGroupController {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	audioStorage := storage.NewAudioStorage(
		cfg.Storage.SpeakerAudioPath,
		cfg.Storage.MaxFileSize,
	)

	return &SpeakerGroupController{
		DB:            db,
		ServiceURL:    cfg.SpeakerService.URL,
		HTTPClient:    httpClient,
		AudioStorage:  audioStorage,
		HistoryConfig: &cfg.History,
	}
}

// CreateSpeakerGroup creates speaker group
func (sgc *SpeakerGroupController) CreateSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	var req struct {
		AgentID     uint    `json:"agent_id" binding:"required"`
		Name        string  `json:"name" binding:"required,min=1,max=100"`
		Prompt      string  `json:"prompt"`
		Description string  `json:"description"`
		TTSConfigID *string `json:"tts_config_id"`
		Voice       *string `json:"voice"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	// Verify agent exists and belongs to current user
	var agent models.Agent
	if err := sgc.DB.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Agent does not exist or no permission to access"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query agent"})
		return
	}

	// Check if speaker group with same name already exists for this user
	var existingGroup models.SpeakerGroup
	if err := sgc.DB.Where("user_id = ? AND name = ?", userID, req.Name).First(&existingGroup).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Speaker group name already exists, please use another name"})
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Create speaker group
	speakerGroup := models.SpeakerGroup{
		UserID:      userID.(uint),
		AgentID:     req.AgentID,
		Name:        req.Name,
		Prompt:      req.Prompt,
		Description: req.Description,
		TTSConfigID: req.TTSConfigID,
		Voice:       req.Voice,
		Status:      "active",
		SampleCount: 0,
	}

	if err := sgc.DB.Create(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create speaker group: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":           speakerGroup.ID,
			"agent_id":     speakerGroup.AgentID,
			"name":         speakerGroup.Name,
			"prompt":       speakerGroup.Prompt,
			"description":  speakerGroup.Description,
			"sample_count": speakerGroup.SampleCount,
			"created_at":   speakerGroup.CreatedAt,
		},
	})
}

// GetSpeakerGroups gets speaker group list
func (sgc *SpeakerGroupController) GetSpeakerGroups(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	// Get query parameters
	agentIDStr := c.Query("agent_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	// Build query
	query := sgc.DB.Model(&models.SpeakerGroup{}).Where("user_id = ?", userID)

	// Filter by agent
	if agentIDStr != "" {
		agentID, err := strconv.ParseUint(agentIDStr, 10, 32)
		if err == nil {
			query = query.Where("agent_id = ?", uint(agentID))
		}
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get data
	var speakerGroups []models.SpeakerGroup
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&speakerGroups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Get agent info (for displaying agent name)
	agentIDs := make([]uint, 0)
	for _, sg := range speakerGroups {
		agentIDs = append(agentIDs, sg.AgentID)
	}

	var agents []models.Agent
	if len(agentIDs) > 0 {
		sgc.DB.Where("id IN ?", agentIDs).Find(&agents)
	}

	agentMap := make(map[uint]string)
	for _, agent := range agents {
		agentMap[agent.ID] = agent.Name
	}

	// Build response
	result := make([]gin.H, 0)
	for _, sg := range speakerGroups {
		result = append(result, gin.H{
			"id":            sg.ID,
			"agent_id":      sg.AgentID,
			"agent_name":    agentMap[sg.AgentID],
			"name":          sg.Name,
			"prompt":        sg.Prompt,
			"description":   sg.Description,
			"tts_config_id": sg.TTSConfigID,
			"voice":         sg.Voice,
			"sample_count":  sg.SampleCount,
			"created_at":    sg.CreatedAt,
			"updated_at":    sg.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": total,
	})
}

// GetSpeakerGroup gets speaker group details (including sample list)
func (sgc *SpeakerGroupController) GetSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	// Query speaker group
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Speaker group does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Query agent info
	var agent models.Agent
	sgc.DB.Where("id = ?", speakerGroup.AgentID).First(&agent)

	// Query sample list
	var samples []models.SpeakerSample
	sgc.DB.Where("speaker_group_id = ?", speakerGroupID).Order("created_at DESC").Find(&samples)

	// Build sample response
	sampleList := make([]gin.H, 0)
	for _, sample := range samples {
		sampleList = append(sampleList, gin.H{
			"id":         sample.ID,
			"uuid":       sample.UUID,
			"file_name":  sample.FileName,
			"file_size":  sample.FileSize,
			"duration":   sample.Duration,
			"file_path":  sample.FilePath,
			"created_at": sample.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":            speakerGroup.ID,
			"agent_id":      speakerGroup.AgentID,
			"agent_name":    agent.Name,
			"name":          speakerGroup.Name,
			"prompt":        speakerGroup.Prompt,
			"description":   speakerGroup.Description,
			"tts_config_id": speakerGroup.TTSConfigID,
			"voice":         speakerGroup.Voice,
			"sample_count":  speakerGroup.SampleCount,
			"samples":       sampleList,
			"created_at":    speakerGroup.CreatedAt,
		},
	})
}

// UpdateSpeakerGroup updates speaker group
func (sgc *SpeakerGroupController) UpdateSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	var req struct {
		AgentID     *uint   `json:"agent_id"`
		Name        string  `json:"name"`
		Prompt      string  `json:"prompt"`
		Description string  `json:"description"`
		TTSConfigID *string `json:"tts_config_id"`
		Voice       *string `json:"voice"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	// Query speaker group
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Speaker group does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// If updating agent ID, need to verify new agent exists
	if req.AgentID != nil && *req.AgentID != speakerGroup.AgentID {
		var agent models.Agent
		if err := sgc.DB.Where("id = ? AND user_id = ?", *req.AgentID, userID).First(&agent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Agent does not exist or no permission to access"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query agent"})
			return
		}
		speakerGroup.AgentID = *req.AgentID
	}

	// Update fields
	if req.Name != "" && req.Name != speakerGroup.Name {
		// Check if speaker group with same name already exists for this user (exclude current speaker group)
		var existingGroup models.SpeakerGroup
		if err := sgc.DB.Where("user_id = ? AND name = ? AND id != ?", userID, req.Name, speakerGroupID).First(&existingGroup).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Speaker group name already exists, please use another name"})
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
			return
		}
		speakerGroup.Name = req.Name
	}
	if req.Prompt != "" {
		speakerGroup.Prompt = req.Prompt
	}
	speakerGroup.Description = req.Description // Allow clearing description
	speakerGroup.TTSConfigID = req.TTSConfigID
	speakerGroup.Voice = req.Voice

	if err := sgc.DB.Save(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update speaker group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    speakerGroup,
	})
}

// DeleteSpeakerGroup deletes speaker group
func (sgc *SpeakerGroupController) DeleteSpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	// Query speaker group
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Speaker group does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Query all samples (for deleting local files and database records)
	var samples []models.SpeakerSample
	sgc.DB.Where("speaker_group_id = ?", speakerGroupID).Find(&samples)

	// Call asr_server delete API (via speaker_id, i.e., speaker group primary key ID, delete all samples at once)
	err = sgc.callDeleteAPI(fmt.Sprintf("%d", speakerGroup.ID), speakerGroup.AgentID, userID)
	if err != nil {
		log.Printf("asr_server failed to delete speaker group (speaker_id: %d): %v", speakerGroup.ID, err)
		// Continue with local deletion, don't interrupt flow
	}

	// Delete all samples' local files and database records
	for _, sample := range samples {
		// Delete local file
		sgc.AudioStorage.DeleteAudioFile(sample.FilePath)

		// Delete database record
		sgc.DB.Delete(&sample)
	}

	// Delete speaker group
	if err := sgc.DB.Delete(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete speaker group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Speaker group deleted successfully",
	})
}

// AddSample adds speaker sample
func (sgc *SpeakerGroupController) AddSample(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	groupIDStr := c.Param("id") // Changed to use :id parameter
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	// Verify speaker group exists and belongs to current user
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", groupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Speaker group does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	var file multipart.File
	var header *multipart.FileHeader
	var fileName string

	// Check if getting audio from chat history
	messageID := c.PostForm("message_id")
	if messageID != "" {
		// Get audio from chat history
		var chatMessage models.ChatMessage
		if err := sgc.DB.Where("message_id = ? AND user_id = ? AND role = ? AND is_deleted = ?",
			messageID, userID, "user", false).First(&chatMessage).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Chat history does not exist or is not a user message"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query chat history"})
			return
		}

		if chatMessage.AudioPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This message has no audio data"})
			return
		}

		// Read audio file
		audioBasePath := sgc.HistoryConfig.AudioBasePath
		if audioBasePath == "" {
			audioBasePath = "./storage/chat_history/audio"
		}
		fullPath := filepath.Join(audioBasePath, chatMessage.AudioPath)

		audioData, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Audio file does not exist"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read audio file: " + err.Error()})
			return
		}

		// Create temp file for multipart
		tempFile, err := os.CreateTemp("", "audio_*.wav")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file: " + err.Error()})
			return
		}
		defer os.Remove(tempFile.Name()) // Clean up temp file
		defer tempFile.Close()

		if _, err := tempFile.Write(audioData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write temp file: " + err.Error()})
			return
		}
		tempFile.Seek(0, 0)

		// Create multipart.File and FileHeader
		file = tempFile
		fileInfo, _ := tempFile.Stat()
		header = &multipart.FileHeader{
			Filename: fmt.Sprintf("history_%s.wav", messageID),
			Size:     fileInfo.Size(),
		}
		fileName = header.Filename
	} else {
		// Get audio from uploaded file
		file, header, err = c.Request.FormFile("audio")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Audio file missing: " + err.Error()})
			return
		}
		defer file.Close()
		fileName = header.Filename
	}

	// Generate UUID
	sampleUUID := uuid.New().String()

	// Save audio file to local
	filePath, savedFileSize, err := sgc.AudioStorage.SaveAudioFile(
		userID.(uint),
		uint(groupID),
		sampleUUID,
		fileName,
		file,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio file: " + err.Error()})
		return
	}

	// Call asr_server register API
	file.Seek(0, 0) // Reset file pointer
	err = sgc.callRegisterAPI(
		fmt.Sprintf("%d", speakerGroup.ID), // speaker_id uses speaker group primary key ID
		speakerGroup.Name,                  // speaker_name uses group name
		sampleUUID,
		speakerGroup.AgentID, // agent_id
		file,
		header,
		userID,
	)
	if err != nil {
		// If registration fails, delete saved file
		sgc.AudioStorage.DeleteAudioFile(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register speaker: " + err.Error()})
		return
	}

	// Create sample record
	sample := models.SpeakerSample{
		SpeakerGroupID: uint(groupID),
		UserID:         userID.(uint),
		UUID:           sampleUUID,
		FilePath:       filePath,
		FileName:       fileName,
		FileSize:       savedFileSize,
		Status:         "active",
	}

	if err := sgc.DB.Create(&sample).Error; err != nil {
		// If database save fails, delete file and asr_server record
		sgc.AudioStorage.DeleteAudioFile(filePath)
		sgc.callDeleteAPI(sampleUUID, speakerGroup.AgentID, userID, sampleUUID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save sample record"})
		return
	}

	// Update speaker group sample count
	sgc.DB.Model(&speakerGroup).Update("sample_count", gorm.Expr("sample_count + 1"))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":         sample.ID,
			"uuid":       sample.UUID,
			"file_name":  sample.FileName,
			"file_size":  sample.FileSize,
			"file_path":  sample.FilePath,
			"created_at": sample.CreatedAt,
		},
	})
}

// GetSamples gets all samples under speaker group
func (sgc *SpeakerGroupController) GetSamples(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	groupIDStr := c.Param("id") // Changed to use :id parameter
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	// Verify speaker group exists and belongs to current user
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", groupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Speaker group does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Query sample list
	var samples []models.SpeakerSample
	if err := sgc.DB.Where("speaker_group_id = ?", groupID).Order("created_at DESC").Find(&samples).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query samples"})
		return
	}

	// Build response
	result := make([]gin.H, 0)
	for _, sample := range samples {
		result = append(result, gin.H{
			"id":         sample.ID,
			"uuid":       sample.UUID,
			"file_name":  sample.FileName,
			"file_size":  sample.FileSize,
			"duration":   sample.Duration,
			"file_path":  sample.FilePath,
			"created_at": sample.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// DeleteSample deletes speaker sample
func (sgc *SpeakerGroupController) DeleteSample(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	groupIDStr := c.Param("id") // Changed to use :id parameter
	sampleIDStr := c.Param("sample_id")

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	sampleID, err := strconv.ParseUint(sampleIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sample ID"})
		return
	}

	// Verify sample exists and belongs to current user
	var sample models.SpeakerSample
	if err := sgc.DB.Where("id = ? AND speaker_group_id = ? AND user_id = ?", sampleID, groupID, userID).First(&sample).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sample does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query sample"})
		return
	}

	// Query speaker group to get AgentID
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ?", groupID).First(&speakerGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Call asr_server delete API (via UUID)
	sgc.callDeleteAPI(sample.UUID, speakerGroup.AgentID, userID, sample.UUID)

	// Delete local file
	sgc.AudioStorage.DeleteAudioFile(sample.FilePath)

	// Delete database record
	if err := sgc.DB.Delete(&sample).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete sample"})
		return
	}

	// Update speaker group sample count
	sgc.DB.Model(&models.SpeakerGroup{}).Where("id = ?", groupID).Update("sample_count", gorm.Expr("sample_count - 1"))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Sample deleted successfully",
	})
}

// VerifySpeakerGroup verifies speaker group
func (sgc *SpeakerGroupController) VerifySpeakerGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	id := c.Param("id")
	speakerGroupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	// Verify speaker group exists and belongs to current user
	var speakerGroup models.SpeakerGroup
	if err := sgc.DB.Where("id = ? AND user_id = ?", speakerGroupID, userID).First(&speakerGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Speaker group does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query speaker group"})
		return
	}

	// Get uploaded audio file
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Audio file missing: " + err.Error()})
		return
	}
	defer file.Close()

	// Call asr_server verify API
	result, err := sgc.callVerifyAPI(fmt.Sprintf("%d", speakerGroup.ID), speakerGroup.AgentID, file, header, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Verification failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"verified":     result.Verified,
			"confidence":   result.Confidence,
			"threshold":    result.Threshold,
			"speaker_id":   fmt.Sprintf("%d", speakerGroup.ID),
			"speaker_name": speakerGroup.Name,
			"message":      sgc.getVerifyMessage(result.Verified, result.Confidence),
		},
	})
}

// getVerifyMessage generates verification result message
func (sgc *SpeakerGroupController) getVerifyMessage(verified bool, confidence float32) string {
	if verified {
		return fmt.Sprintf("Verification passed, similarity: %.1f%%", confidence*100)
	}
	return fmt.Sprintf("Verification failed, similarity: %.1f%%", confidence*100)
}

// VerifyResult verification result
type VerifyResult struct {
	SpeakerID   string  `json:"speaker_id"`
	SpeakerName string  `json:"speaker_name"`
	Verified    bool    `json:"verified"`
	Confidence  float32 `json:"confidence"`
	Threshold   float32 `json:"threshold"`
}

// callVerifyAPI calls asr_server verify API
func (sgc *SpeakerGroupController) callVerifyAPI(speakerID string, agentID uint, file multipart.File, header *multipart.FileHeader, userID interface{}) (*VerifyResult, error) {
	// Prepare multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add file
	part, err := writer.CreateFormFile("audio", header.Filename)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("Failed to create file field: %v", err)
	}

	// Reset file pointer
	file.Seek(0, 0)
	if _, err := io.Copy(part, file); err != nil {
		writer.Close()
		return nil, fmt.Errorf("Failed to copy file content: %v", err)
	}

	writer.Close()

	// Create request
	apiURL := fmt.Sprintf("%s/api/v1/speaker/verify/%s", sgc.ServiceURL, url.PathEscape(speakerID))
	req, err := http.NewRequest("POST", apiURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	req.Header.Set("X-Agent-ID", fmt.Sprintf("%d", agentID)) // Add agent_id header

	// Send request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := sgc.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asr_server returned error (status code: %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result VerifyResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Failed to parse response: %v", err)
	}

	return &result, nil
}

// GetSampleFile gets sample audio file
func (sgc *SpeakerGroupController) GetSampleFile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	groupIDStr := c.Param("id")
	sampleIDStr := c.Param("sample_id")

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid speaker group ID"})
		return
	}

	sampleID, err := strconv.ParseUint(sampleIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sample ID"})
		return
	}

	// Verify sample exists and belongs to current user
	var sample models.SpeakerSample
	if err := sgc.DB.Where("id = ? AND speaker_group_id = ? AND user_id = ?", sampleID, groupID, userID).First(&sample).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sample does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query sample"})
		return
	}

	// Check if file exists
	if !sgc.AudioStorage.FileExists(sample.FilePath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audio file does not exist"})
		return
	}

	// Open file
	file, err := sgc.AudioStorage.GetAudioFile(sample.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file info"})
		return
	}

	// Set response headers
	c.Header("Content-Type", "audio/wav")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", sample.FileName))
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Return file content
	c.File(sample.FilePath)
}

// callRegisterAPI calls asr_server register API
func (sgc *SpeakerGroupController) callRegisterAPI(speakerID, speakerName, uuid string, agentID uint, file multipart.File, header *multipart.FileHeader, userID interface{}) error {
	// Prepare multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add fields
	writer.WriteField("speaker_id", speakerID)
	writer.WriteField("speaker_name", speakerName)
	writer.WriteField("uuid", uuid)
	writer.WriteField("agent_id", fmt.Sprintf("%d", agentID)) // Add agent_id field
	writer.WriteField("uid", fmt.Sprintf("%v", userID))

	// Add file
	part, err := writer.CreateFormFile("audio", header.Filename)
	if err != nil {
		writer.Close()
		return fmt.Errorf("Failed to create file field: %v", err)
	}

	// Reset file pointer
	file.Seek(0, 0)
	if _, err := io.Copy(part, file); err != nil {
		writer.Close()
		return fmt.Errorf("Failed to copy file content: %v", err)
	}

	writer.Close()

	// Create request
	url := fmt.Sprintf("%s/api/v1/speaker/register", sgc.ServiceURL)
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	req.Header.Set("X-Agent-ID", fmt.Sprintf("%d", agentID)) // Add agent_id header

	// Send request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := sgc.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("asr_server returned error: %s", string(body))
	}

	return nil
}

// callDeleteAPI calls asr_server delete API
// speakerID: used as path parameter (speaker_id or uuid)
// agentID: Agent ID
// uuid: optional, if provided used as query parameter (for deleting single sample)
func (sgc *SpeakerGroupController) callDeleteAPI(speakerID string, agentID uint, userID interface{}, uuid ...string) error {
	// Build URL: path parameter uses speakerID
	apiURL := fmt.Sprintf("%s/api/v1/speaker/%s", sgc.ServiceURL, url.PathEscape(speakerID))

	// Build query parameters
	queryParams := make([]string, 0)
	if len(uuid) > 0 && uuid[0] != "" {
		queryParams = append(queryParams, fmt.Sprintf("uuid=%s", url.QueryEscape(uuid[0])))
	}
	queryParams = append(queryParams, fmt.Sprintf("agent_id=%d", agentID))

	if len(queryParams) > 0 {
		apiURL += "?" + strings.Join(queryParams, "&")
	}

	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	req.Header.Set("X-Agent-ID", fmt.Sprintf("%d", agentID)) // Add agent_id header

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := sgc.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(uuid) > 0 && uuid[0] != "" {
			log.Printf("asr_server delete failed (speaker_id: %s, uuid: %s): %s", speakerID, uuid[0], string(body))
		} else {
			log.Printf("asr_server delete failed (speaker_id: %s): %s", speakerID, string(body))
		}
		// If uuid is provided, don't return error (may already be deleted or not exist)
		// If deleting via speaker_id, return error
		if len(uuid) == 0 || uuid[0] == "" {
			return fmt.Errorf("asr_server returned error: %s", string(body))
		}
	}

	return nil
}
