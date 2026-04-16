package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	log "xiaozhi-esp32-server-golang/logger"
)

var (
	// global Redis client instance
	globalClient *redis.Client
	// ensure only initialize once
	once sync.Once
	// read-write lock protected instance access
	mu sync.RWMutex
)

// Config Redis config struct
type Config struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	Password string `mapstructure:"password" json:"password"`
	DB       int    `mapstructure:"db" json:"db"`
	// connection pool config
	PoolSize     int           `mapstructure:"pool_size" json:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns" json:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries" json:"max_retries"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" json:"dial_timeout"`
}

// DefaultConfig returndefaultconfig
func DefaultConfig() *Config {
	return &Config{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		DialTimeout:  5 * time.Second,
	}
}

// Init initialize Redis client
func Init(config *Config) error {
	var initErr error

	once.Do(func() {
		if config == nil {
			config = DefaultConfig()
		}

		// create Redis client
		options := &redis.Options{
			Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
			Password:     config.Password,
			DB:           config.DB,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxRetries:   config.MaxRetries,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			DialTimeout:  config.DialTimeout,
		}

		client := redis.NewClient(options)

		// test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("failed to connect to redis: %w", err)
			return
		}

		mu.Lock()
		globalClient = client
		mu.Unlock()

		log.Log().Info("Redis client initialized successfully")
	})

	return initErr
}

// GetClient get Redis client instance
func GetClient() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()

	if globalClient == nil {
		log.Log().Warn("Redis client not initialized")
		return nil
	}

	return globalClient
}

// GetClientWithOptions use specified config to get Redis client
func GetClientWithOptions(options *redis.Options) *redis.Client {
	if options == nil {
		return GetClient()
	}

	client := redis.NewClient(options)

	// test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Log().Errorf("Redis connection failed: %v", err)
		return nil
	}

	return client
}

// IsHealthy check Redis connection health status
func IsHealthy() bool {
	client := GetClient()
	if client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return client.Ping(ctx).Err() == nil
}

// Close close Redis client connection
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		err := globalClient.Close()
		globalClient = nil
		if err != nil {
			log.Log().Errorf("close Redis connection failed: %v", err)
			return err
		}
		log.Log().Info("Redis connection closed")
	}

	return nil
}

// GetKeyWithPrefix get key name with prefix
func GetKeyWithPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", prefix, key)
}

// Reconnect reconnect to Redis (used for reconnecting after connection disconnect)
func Reconnect() error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		// close existing connection
		_ = globalClient.Close()
		globalClient = nil
	}

	// reset once, allow reinitialize
	once = sync.Once{}

	return nil
}

// Stats get Redis connection pool stats info
func Stats() *redis.PoolStats {
	client := GetClient()
	if client == nil {
		return nil
	}

	stats := client.PoolStats()
	return stats
}

// LogStats record Redis connection pool stats info
func LogStats() {
	stats := Stats()
	if stats == nil {
		log.Log().Warn("unable to get Redis connection pool stats info")
		return
	}

	log.Log().Infof("Redis connection pool stats - total connections: %d, idle connections: %d, stale connections: %d, hits: %d, misses: %d, timeouts: %d",
		stats.TotalConns, stats.IdleConns, stats.StaleConns, stats.Hits, stats.Misses, stats.Timeouts)
}
