package storage

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// GormBaseStorage generic GORM storage base class
// contains common implementations for all GORM-based storage operations
type GormBaseStorage struct {
	DB *gorm.DB // exported field, allows subclasses to access
}

// NewGormBaseStorage create GORM base storage instance
func NewGormBaseStorage(db *gorm.DB) *GormBaseStorage {
	return &GormBaseStorage{
		DB: db,
	}
}

// Ping check database connection
func (s *GormBaseStorage) Ping() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Ping()
}

// Close close database connection
func (s *GormBaseStorage) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// BeginTx start transaction
func (s *GormBaseStorage) BeginTx(ctx context.Context) (Transaction, error) {
	tx := s.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	transaction := &GormTransaction{
		DB: tx,
	}
	transaction.init()
	return transaction, nil
}

// GormTransaction generic GORM transaction implementation
type GormTransaction struct {
	DB *gorm.DB
	*GormUserStorage
	*GormDeviceStorage
	*GormAgentStorage
	*GormConfigStorage
}

// init initialize storage components in transaction
func (t *GormTransaction) init() {
	t.GormUserStorage = &GormUserStorage{db: t.DB}
	t.GormDeviceStorage = &GormDeviceStorage{db: t.DB}
	t.GormAgentStorage = &GormAgentStorage{db: t.DB}
	t.GormConfigStorage = &GormConfigStorage{db: t.DB}
}

// Commit commit transaction
func (t *GormTransaction) Commit() error {
	if err := t.DB.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rollback transaction
func (t *GormTransaction) Rollback() error {
	if err := t.DB.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}
