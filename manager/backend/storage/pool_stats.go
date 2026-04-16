package storage

import (
	"sync"
	"time"
)

// PoolStatsData resource pool statistics data
type PoolStatsData struct {
	Timestamp time.Time              `json:"timestamp"`
	Stats     map[string]interface{} `json:"stats"`
}

// PoolStatsStorage resource pool statistics storage (in-memory storage, only saves latest data)
type PoolStatsStorage struct {
	mu     sync.RWMutex
	latest *PoolStatsData // only saves the latest statistics data
}

var (
	globalPoolStatsStorage *PoolStatsStorage
	once                   sync.Once
)

// GetPoolStatsStorage get global resource pool statistics storage (singleton)
func GetPoolStatsStorage() *PoolStatsStorage {
	once.Do(func() {
		globalPoolStatsStorage = &PoolStatsStorage{
			latest: nil,
		}
	})
	return globalPoolStatsStorage
}

// AddStats add statistics data (only saves latest, overwrites old data)
func (s *PoolStatsStorage) AddStats(stats map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// directly overwrite latest data
	s.latest = &PoolStatsData{
		Timestamp: time.Now(),
		Stats:     stats,
	}
}

// GetLatestStats get the latest statistics data
func (s *PoolStatsStorage) GetLatestStats() *PoolStatsData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return nil
	}

	// return a copy of the latest data
	latest := *s.latest
	return &latest
}

// GetAllStats get all statistics data (only returns the latest one)
func (s *PoolStatsStorage) GetAllStats() []PoolStatsData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return []PoolStatsData{}
	}

	// only return the latest data
	return []PoolStatsData{*s.latest}
}

// GetStatsByTimeRange get statistics data by time range (only returns latest data if within time range)
func (s *PoolStatsStorage) GetStatsByTimeRange(start, end time.Time) []PoolStatsData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == nil {
		return []PoolStatsData{}
	}

	// check if latest data is within time range
	if s.latest.Timestamp.After(start) && s.latest.Timestamp.Before(end) {
		return []PoolStatsData{*s.latest}
	}

	return []PoolStatsData{}
}

// GetStatsCount get current number of stored data entries (only saves latest data, so returns 0 or 1)
func (s *PoolStatsStorage) GetStatsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest == nil {
		return 0
	}
	return 1
}
