package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Config struct {
	Server            ServerConfig         `json:"server"`
	Database          DatabaseConfig       `json:"database"`
	JWT               JWTConfig            `json:"jwt"`
	InternalAuthToken string               `json:"internal_auth_token"`
	EndpointAuthToken string               `json:"endpoint_auth_token"`
	SpeakerService    SpeakerServiceConfig `json:"speaker_service"`
	Storage           StorageConfig        `json:"storage"`
	History           HistoryConfig        `json:"history"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Mode string `json:"mode"`
}

type DatabaseConfig struct {
	Type   string        `json:"type"` // "mysql" or "sqlite", determines which database to use
	MySQL  *MySQLConfig  `json:"mysql,omitempty"`
	SQLite *SQLiteConfig `json:"sqlite,omitempty"`
}

// GetStorageType gets the currently configured storage type
func (c *DatabaseConfig) GetStorageType() string {
	if c.Type == "sqlite" || c.Type == "mysql" {
		return c.Type
	}
	// When type is not set, infer from existing configuration
	if c.SQLite != nil {
		return "sqlite"
	}
	if c.MySQL != nil {
		return "mysql"
	}
	return "mysql"
}

// MySQLConfig MySQL database configuration
type MySQLConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// SQLiteConfig SQLite database configuration
type SQLiteConfig struct {
	FilePath string `json:"file_path"` // Database file path, e.g. ./data/xiaozhi.db
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	ExpireHour int    `json:"expire_hour"`
}

type SpeakerServiceConfig struct {
	URL string `json:"url"` // asr_server service address
}

type StorageConfig struct {
	SpeakerAudioPath string `json:"speaker_audio_path"` // Audio file storage path
	MaxFileSize      int64  `json:"max_file_size"`      // Maximum file size (bytes), default 10MB
}

type HistoryConfig struct {
	Enabled       bool   `json:"enabled"`
	AudioBasePath string `json:"audio_base_path"` // Audio storage base path
	MaxFileSize   int64  `json:"max_file_size"`   // Maximum file size (bytes), default 10MB
}

func Load() *Config {
	return LoadWithPath("config/config.json")
}

func LoadWithPath(configPath string) *Config {
	config := LoadFromFile(configPath)

	// Only when using MySQL, ensure MySQL config exists and apply environment variable overrides
	if config.Database.GetStorageType() == "mysql" {
		if config.Database.MySQL == nil {
			config.Database.MySQL = &MySQLConfig{}
		}
		if host := os.Getenv("DB_HOST"); host != "" {
			config.Database.MySQL.Host = host
		}
		if port := os.Getenv("DB_PORT"); port != "" {
			var p int
			fmt.Sscanf(port, "%d", &p)
			config.Database.MySQL.Port = p
		}
		if username := os.Getenv("DB_USER"); username != "" {
			config.Database.MySQL.Username = username
		}
		if password := os.Getenv("DB_PASSWORD"); password != "" {
			config.Database.MySQL.Password = password
		}
		if database := os.Getenv("DB_NAME"); database != "" {
			config.Database.MySQL.Database = database
		}
	}

	// Priority use environment variables to override speaker service configuration
	if serviceURL := os.Getenv("SPEAKER_SERVICE_URL"); serviceURL != "" {
		config.SpeakerService.URL = serviceURL
	}
	// Priority use environment variables to override audio storage path
	if audioBasePath := os.Getenv("AUDIO_BASE_PATH"); audioBasePath != "" {
		config.History.AudioBasePath = audioBasePath
	}

	fmt.Println("config", config)

	return config
}

func LoadFromFile(configPath string) *Config {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Failed to open configuration file %s: %v", configPath, err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("Failed to parse configuration file %s: %v", configPath, err)
	}

	return &config
}
