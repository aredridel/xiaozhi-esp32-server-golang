package controllers

import (
	"log"
	"net/http"
	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SetupController struct {
	DB *gorm.DB
}

type SetupRequest struct {
	AdminUsername string `json:"admin_username" binding:"required,min=3,max=50"`
	AdminPassword string `json:"admin_password" binding:"required,min=6,max=100"`
	AdminEmail    string `json:"admin_email" binding:"required,email"`
}

// Check if database needs initialization
func (sc *SetupController) CheckSetupStatus(c *gin.Context) {
	if sc.DB == nil {
		c.JSON(http.StatusOK, gin.H{
			"needs_setup": true,
			"message":     "Database connection unavailable",
		})
		return
	}

	// Check if user table exists
	if !sc.DB.Migrator().HasTable(&models.User{}) {
		c.JSON(http.StatusOK, gin.H{
			"needs_setup": true,
			"message":     "Database table structure not initialized",
		})
		return
	}

	// Check if admin user exists
	var count int64
	sc.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&count)

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"needs_setup": true,
			"message":     "Need to create admin account",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"needs_setup": false,
		"message":     "System initialized",
	})
}

// Initialize database
func (sc *SetupController) InitializeDatabase(c *gin.Context) {
	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if sc.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection unavailable"})
		return
	}

	// Begin transaction
	tx := sc.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start database transaction"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Auto-migrate table structure
	log.Println("Starting database table structure auto-migration...")
	err := tx.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Agent{},
		&models.Config{},
		&models.MCPMarketService{},
		&models.GlobalRole{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
		&models.ChatMessage{},
	)
	if err != nil {
		tx.Rollback()
		log.Printf("Database table structure migration failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database table structure migration failed: " + err.Error()})
		return
	}
	log.Println("Database table structure migration successful")

	// 2. Check if admin user already exists
	var existingAdmin models.User
	if err := tx.Where("role = ?", "admin").First(&existingAdmin).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin user already exists, cannot reinitialize"})
		return
	}

	// 3. Check if username already exists
	var existingUser models.User
	if err := tx.Where("username = ?", req.AdminUsername).First(&existingUser).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	// 4. Check if email already exists
	if err := tx.Where("email = ?", req.AdminEmail).First(&existingUser).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
		return
	}

	// 5. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password encryption failed"})
		return
	}

	// 6. Create admin user
	admin := models.User{
		Username: req.AdminUsername,
		Password: string(hashedPassword),
		Email:    req.AdminEmail,
		Role:     "admin",
	}

	if err := tx.Create(&admin).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to create admin user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create admin user: " + err.Error()})
		return
	}

	// 7. Create some default global roles
	defaultRoles := []models.GlobalRole{
		{
			Name:        "Assistant",
			Description: "A friendly AI assistant that helps users solve various problems",
			Prompt:      "You are a friendly and professional AI assistant. Please answer user questions in clear and concise language, and provide helpful suggestions.",
			IsDefault:   true,
		},
		{
			Name:        "Teacher",
			Description: "A patient teacher who can explain complex concepts in detail",
			Prompt:      "You are an experienced teacher. Please explain complex concepts in an easy-to-understand way, and give specific examples to help with understanding.",
			IsDefault:   false,
		},
		{
			Name:        "Friend",
			Description: "A caring friend who can listen and accompany",
			Prompt:      "You are a caring friend. Please communicate with users in a warm and understanding manner, providing emotional support and encouragement.",
			IsDefault:   false,
		},
	}

	for _, role := range defaultRoles {
		if err := tx.Create(&role).Error; err != nil {
			log.Printf("Failed to create default role: %v", err)
			// Don't interrupt initialization process, continue
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit database transaction"})
		return
	}

	log.Printf("Database initialization successful, admin user: %s", req.AdminUsername)
	c.JSON(http.StatusOK, gin.H{
		"message": "Database initialization successful",
		"admin": gin.H{
			"username": admin.Username,
			"email":    admin.Email,
			"role":     admin.Role,
		},
	})
}
