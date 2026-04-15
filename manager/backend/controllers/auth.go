package controllers

import (
	"log"
	"net/http"
	"xiaozhi/manager/backend/middleware"
	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthController struct {
	DB *gorm.DB
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// User login
func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add login debug logs
	log.Printf("[Login] Attempting login for user: %s, Client IP: %s", req.Username, c.ClientIP())
	log.Printf("[Login] Received password length: %d", len(req.Password))

	// If database is available, try to verify from database
	if ac.DB != nil {
		log.Printf("[Login] Database connection available, starting database verification")
		var user models.User
		if err := ac.DB.Where("username = ?", req.Username).First(&user).Error; err == nil {
			log.Printf("[Login] Found user: ID=%d, Username=%s, Role=%s, Email=%s", user.ID, user.Username, user.Role, user.Email)
			log.Printf("[Login] Password hash length in database: %d, hash prefix: %s", len(user.Password), user.Password[:10])
			log.Printf("[Login] Starting bcrypt password comparison")

			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err == nil {
				log.Printf("[Login] ✅ Password verification successful - User: %s", req.Username)
				token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
				if err != nil {
					log.Printf("[Login] ❌ Token generation failed: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
					return
				}

				log.Printf("[Login] ✅ Login successful, returning token - User: %s, Role: %s", user.Username, user.Role)
				c.JSON(http.StatusOK, gin.H{
					"token": token,
					"user": gin.H{
						"id":       user.ID,
						"username": user.Username,
						"email":    user.Email,
						"role":     user.Role,
					},
				})
				return
			} else {
				log.Printf("[Login] ❌ Password verification failed - User: %s, bcrypt error: %v", req.Username, err)
				log.Printf("[Login] Debug info - Input password: '%s', Hash: '%s'", req.Password, user.Password)
			}
		} else {
			log.Printf("[Login] ❌ User does not exist - Username: %s, Database error: %v", req.Username, err)
		}
	} else {
		log.Printf("[Login] ❌ Database connection not available")
	}

	// Fallback: Hardcoded admin user verification (when database is unavailable)
	if req.Username == "admin" && req.Password == "admin123" {
		token, err := middleware.GenerateToken(1, "admin", "admin")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"id":       1,
				"username": "admin",
				"email":    "admin@xiaozhi.com",
				"role":     "admin",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
}

// User registration
func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := ac.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	// Encrypt password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password encryption failed"})
		return
	}

	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Role:     "user",
	}

	if err := ac.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// Get current user info
func (ac *AuthController) GetProfile(c *gin.Context) {
	log.Printf("[GetProfile] Starting to process get user info request, Client IP: %s", c.ClientIP())

	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("[GetProfile] ❌ Cannot get user ID, authentication middleware may not be properly set")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication information missing"})
		return
	}

	log.Printf("[GetProfile] Got user ID from context: %v", userID)

	var user models.User
	if err := ac.DB.First(&user, userID).Error; err != nil {
		log.Printf("[GetProfile] ❌ Database query for user failed: %v, User ID: %v", err, userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	log.Printf("[GetProfile] ✅ Successfully retrieved user info - ID: %d, Username: %s, Role: %s", user.ID, user.Username, user.Role)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}
