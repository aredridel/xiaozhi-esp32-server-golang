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
	// globalRedisclient-sideinstance
	globalClient *redis.Client
	// ensureonlyinitializeatimes
	once sync.Once
	// readwritelockprotectedinstanceaccess
	mu sync.RWMutex
)

// Config Redisconfigstructurebody
type Config struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	Password string `mapstructure:"password" json:"password"`
	DB       int    `mapstructure:"db" json:"db"`
	// joinpoolconfig
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

// Init initializeRedisclient-side
func Init(config *Config) error {
	var initErr error

	once.Do(func() {
		if config == nil {
			config = DefaultConfig()
		}

		// createRedisclient-side
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

		// testjoin
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("failed to connect to redis: %w", err)
			return
		}

		mu.Lock()
		globalClient = client
		mu.Unlock()

		log.Log().Info("Redisclient-sideinitializesuccessful")
	})

	return initErr
}

// GetClient getRedisclient-sideinstance
func GetClient() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()

	if globalClient == nil {
		log.Log().Warn("Redisclient-sidenot initialized")
		return nil
	}

	return globalClient
}

// GetClientWithOptions usespecifyconfiggetRedisclient-side
func GetClientWithOptions(options *redis.Options) *redis.Client {
	if options == nil {
		return GetClient()
	}

	client := redis.NewClient(options)

	// testjoin
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Log().Errorf("Redisjoinfailed: %v", err)
		return nil
	}

	return client
}

// IsHealthy inspectRedisjoin健康state
func IsHealthy() bool {
	client := GetClient()
	if client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return client.Ping(ctx).Err() == nil
}

// Close closeRedisclient-sidejoin
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		err := globalClient.Close()
		globalClient = nil
		if err != nil {
			log.Log().Errorf("closeRedisjoinfailed: %v", err)
			return err
		}
		log.Log().Info("Redisjoinalreadyclose")
	}

	return nil
}

// GetKeyWithPrefix get带before缀ofkeyname
func GetKeyWithPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", prefix, key)
}

// Reconnect rejoinRedis（used forjoindisconnectafterofreconnect）
func Reconnect() error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		// close现havejoin
		_ = globalClient.Close()
		globalClient = nil
	}

	// resetonce，allowreinitialize
	once = sync.Once{}

	return nil
}

// Stats getRedisjoinpoolcountinfo
func Stats() *redis.PoolStats {
	client := GetClient()
	if client == nil {
		return nil
	}

	stats := client.PoolStats()
	return stats
}

// LogStats recordRedisjoinpoolcountinfo
func LogStats() {
	stats := Stats()
	if stats == nil {
		log.Log().Warn("no法getRedisjoinpoolcountinfo")
		return
	}

	log.Log().Infof("Redisjoinpoolcount - 总join: %d, empty闲join: %d, expirejoin: %d, 命in: %d, not命in: %d, timeout: %d",
		stats.TotalConns, stats.IdleConns, stats.StaleConns, stats.Hits, stats.Misses, stats.Timeouts)
}
