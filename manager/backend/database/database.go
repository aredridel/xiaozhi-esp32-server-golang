package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) *gorm.DB {
	var db *gorm.DB
	var err error

	storageType := cfg.GetStorageType()

	if storageType == "sqlite" {
		if cfg.SQLite == nil {
			log.Println("SQLite config is empty, will run in fallback mode (hardcoded user validation)")
			return nil
		}
		// Ensure database file directory exists to avoid SQLite "unable to open database file" error
		dir := filepath.Dir(cfg.SQLite.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Failed to create database directory %s: %v", dir, err)
			return nil
		}
		log.Println("Using SQLite database:", cfg.SQLite.FilePath)
		db, err = gorm.Open(sqlite.Open(cfg.SQLite.FilePath), &gorm.Config{})
	} else {
		if cfg.MySQL == nil {
			log.Println("MySQL config is empty, will run in fallback mode (hardcoded user validation)")
			return nil
		}
		// MySQL database connection
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Println("Database connection failed:", err)
		log.Println("Will run in fallback mode (hardcoded user validation)")
		return nil
	}

	log.Println("Database connection successful")

	// Auto migrate database table structure
	log.Println("Starting auto migration of database table structure...")
	err = db.AutoMigrate(
		&models.User{},
		&models.APIToken{},
		&models.Device{},
		&models.Agent{},
		&models.KnowledgeBase{},
		&models.KnowledgeBaseDocument{},
		&models.AgentKnowledgeBase{},
		&models.Config{},
		&models.MCPMarketService{},
		&models.GlobalRole{},
		&models.Role{}, // New: unified role table
		&models.ChatMessage{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
		&models.VoiceClone{},
		&models.VoiceCloneAudio{},
		&models.VoiceCloneTask{},
		&models.UserVoiceCloneQuota{},
	)
	if err != nil {
		log.Printf("Database table structure migration failed: %v", err)
		log.Println("Will run in fallback mode (hardcoded user validation)")
		return nil
	}
	log.Println("Database table structure migration successful")

	// Migrate existing global role data to new roles table
	log.Println("Checking if global role data migration is needed...")
	if err := migrateGlobalRolesToRoles(db); err != nil {
		log.Printf("Global role data migration failed: %v", err)
		// Migration failure doesn't affect startup, just means data wasn't migrated
	}

	return db
}

func Close(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Println("Failed to get database connection:", err)
		return
	}
	sqlDB.Close()
}

// migrateGlobalRolesToRoles migrates existing global role data to new roles table
func migrateGlobalRolesToRoles(db *gorm.DB) error {
	// Check if roles table already has data
	var count int64
	if err := db.Table("roles").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check roles table: %w", err)
	}

	// If roles table already has data, skip migration
	if count > 0 {
		log.Println("roles table already has data, skipping migration")
		return nil
	}

	// Check if global_roles table has data
	var globalRoleCount int64
	if err := db.Table("global_roles").Count(&globalRoleCount).Error; err != nil {
		// global_roles table may not exist, not an error
		log.Println("global_roles table does not exist, skipping migration")
		return nil
	}

	if globalRoleCount == 0 {
		log.Println("global_roles table has no data, skipping migration")
		return nil
	}

	log.Printf("Starting migration of %d global role records to roles table...", globalRoleCount)

	// Query all global roles
	var globalRoles []models.GlobalRole
	if err := db.Table("global_roles").Find(&globalRoles).Error; err != nil {
		return fmt.Errorf("failed to query global_roles: %w", err)
	}

	// Convert and insert into roles table
	for _, gr := range globalRoles {
		role := models.Role{
			UserID:      nil, // Global roles have user_id as NULL
			Name:        gr.Name,
			Description: gr.Description,
			Prompt:      gr.Prompt,
			RoleType:    "global",
			Status:      "active",
			SortOrder:   0,
			IsDefault:   gr.IsDefault,
			CreatedAt:   gr.CreatedAt,
			UpdatedAt:   gr.UpdatedAt,
		}
		if err := db.Create(&role).Error; err != nil {
			log.Printf("Failed to insert role %s: %v", gr.Name, err)
			continue
		}
		log.Printf("Migrated global role: %s", gr.Name)
	}

	log.Println("Global role data migration completed")
	return nil
}
