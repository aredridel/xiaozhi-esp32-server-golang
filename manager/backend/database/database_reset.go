package database

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitWithReset initializes database and resets all tables (for development environment only)
func InitWithReset(cfg config.DatabaseConfig) *gorm.DB {
	storageType := cfg.GetStorageType()
	var db *gorm.DB
	var err error

	if storageType == "sqlite" {
		if cfg.SQLite == nil {
			log.Fatal("SQLite config is empty")
		}
		db, err = gorm.Open(sqlite.Open(cfg.SQLite.FilePath), &gorm.Config{})
	} else {
		if cfg.MySQL == nil {
			log.Fatal("MySQL config is empty")
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	log.Println("Warning: Resetting database tables, all data will be deleted!")

	// Drop all tables
	err = db.Migrator().DropTable(
		&models.User{},
		&models.Device{},
		&models.Agent{},
		&models.Config{},
		&models.MCPMarketService{},
		&models.GlobalRole{},
		&models.Role{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
		&models.VoiceClone{},
		&models.VoiceCloneAudio{},
	)
	if err != nil {
		log.Printf("Error dropping tables (tables may not exist): %v", err)
	}

	log.Println("Database tables deletion completed!")
	return db
}
