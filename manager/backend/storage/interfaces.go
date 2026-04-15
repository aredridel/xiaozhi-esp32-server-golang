package storage

import (
	"context"
	"xiaozhi/manager/backend/models"
)

// Storage general storage interface
type Storage interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error

	// Transaction management
	BeginTx(ctx context.Context) (Transaction, error)

	// User management
	UserStorage
	// Device management
	DeviceStorage
	// Agent management
	AgentStorage
	// Configuration management
	ConfigStorage
}

// Transaction transaction interface
type Transaction interface {
	Commit() error
	Rollback() error
	// Execute storage operations within transaction
	UserStorage
	DeviceStorage
	AgentStorage
	ConfigStorage
}

// UserStorage user storage interface
type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByID(ctx context.Context, id uint) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUsers(ctx context.Context, offset, limit int) ([]*models.User, int64, error)
	UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteUser(ctx context.Context, id uint) error
}

// DeviceStorage device storage interface
type DeviceStorage interface {
	CreateDevice(ctx context.Context, device *models.Device) error
	GetDeviceByID(ctx context.Context, id uint) (*models.Device, error)
	GetDeviceByCode(ctx context.Context, deviceCode string) (*models.Device, error)
	GetDevicesByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Device, int64, error)
	UpdateDevice(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteDevice(ctx context.Context, id uint) error
}

// AgentStorage agent storage interface
type AgentStorage interface {
	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, id uint) (*models.Agent, error)
	GetAgentsByUserID(ctx context.Context, userID uint, offset, limit int) ([]*models.Agent, int64, error)
	UpdateAgent(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteAgent(ctx context.Context, id uint) error
}

// ConfigStorage configuration storage interface
type ConfigStorage interface {
	// General configuration operations
	CreateConfig(ctx context.Context, config *models.Config) error
	GetConfigs(ctx context.Context, configType string) ([]*models.Config, error)
	GetConfigByID(ctx context.Context, id uint) (*models.Config, error)
	GetConfigByTypeAndName(ctx context.Context, configType, name string) (*models.Config, error)
	GetDefaultConfig(ctx context.Context, configType string) (*models.Config, error)
	UpdateConfig(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteConfig(ctx context.Context, id uint) error
	SetDefaultConfig(ctx context.Context, configType string, id uint) error

	// Global role configuration
	CreateGlobalRole(ctx context.Context, role *models.GlobalRole) error
	GetGlobalRoles(ctx context.Context) ([]*models.GlobalRole, error)
	GetGlobalRoleByID(ctx context.Context, id uint) (*models.GlobalRole, error)
	UpdateGlobalRole(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteGlobalRole(ctx context.Context, id uint) error
}

// StorageConfig storage configuration interface
type StorageConfig interface {
	GetType() string
	Validate() error
}
