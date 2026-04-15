package storage

import (
	"fmt"

	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/storage/mysql"
	"xiaozhi/manager/backend/storage/sqlite"
)

// StorageType storage type
type StorageType string

const (
	StorageTypeMySQL  StorageType = "mysql"
	StorageTypeSQLite StorageType = "sqlite"
)

// Factory storage factory
type Factory struct{}

// NewFactory creates a storage factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateStorage creates a storage instance
func CreateStorage(dbConfig config.DatabaseConfig) (*StorageAdapter, error) {
	// Determine storage type based on configuration
	storageType := dbConfig.GetStorageType()

	switch StorageType(storageType) {
	case StorageTypeSQLite:
		if dbConfig.SQLite == nil {
			return nil, fmt.Errorf("SQLite config is required")
		}
		// Validate SQLite configuration
		if err := sqlite.ValidateConfig(dbConfig.SQLite); err != nil {
			return nil, fmt.Errorf("invalid SQLite config: %w", err)
		}
		// Create SQLite configuration
		sqliteConfig := sqlite.NewConfigFromDatabase(dbConfig.SQLite)
		// Create SQLite storage
		sqliteStorage, err := sqlite.NewStorage(sqliteConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQLite storage: %w", err)
		}
		// Create base storage
		baseStorage := NewGormBaseStorage(sqliteStorage.DB)
		// Return adapter
		return NewStorageAdapter(baseStorage), nil

	case StorageTypeMySQL:
		if dbConfig.MySQL == nil {
			return nil, fmt.Errorf("MySQL config is required")
		}
		// Validate MySQL configuration
		if err := mysql.ValidateConfig(dbConfig.MySQL); err != nil {
			return nil, fmt.Errorf("invalid MySQL config: %w", err)
		}
		// Create MySQL configuration
		mysqlConfig := mysql.NewConfigFromDatabase(dbConfig.MySQL)
		// Create MySQL storage
		mysqlStorage, err := mysql.NewStorage(mysqlConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create MySQL storage: %w", err)
		}
		// Create base storage
		baseStorage := NewGormBaseStorage(mysqlStorage.DB)
		// Return adapter
		return NewStorageAdapter(baseStorage), nil

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// GetSupportedTypes gets supported storage types
func (f *Factory) GetSupportedTypes() []StorageType {
	return []StorageType{
		StorageTypeMySQL,
		StorageTypeSQLite,
	}
}
