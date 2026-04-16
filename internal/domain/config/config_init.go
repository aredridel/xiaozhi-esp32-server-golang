package user_config

import (
	"context"
	"fmt"
	log "xiaozhi-esp32-server-golang/logger"

	"xiaozhi-esp32-server-golang/internal/domain/config/manager"
	"xiaozhi-esp32-server-golang/internal/domain/config/memory"
	redis_config "xiaozhi-esp32-server-golang/internal/domain/config/redis"

	"github.com/spf13/viper"
)

var (
	// managerSystemConfigHandlers receive WebSocket system_config push callback list, main program can register multiple times (e.g. merge to viper, hot update service)
	managerSystemConfigHandlers []func(map[string]interface{})
)

// RegisterManagerSystemConfigHandler register manager pattern down system config push callback, should be called before InitConfigSystem; can be called multiple times to append multiple callbacks
func RegisterManagerSystemConfigHandler(fn func(map[string]interface{})) {
	managerSystemConfigHandlers = append(managerSystemConfigHandlers, fn)
}

// InitConfigSystem initialize config system
// according to config_provider.type value call corresponding config package Init method
func InitConfigSystem(ctx context.Context) error {
	// get config provider type
	providerType := viper.GetString("config_provider.type")
	if providerType == "" {
		providerType = "redis" // default use redis
		log.Infof("config_provider.type not set, using default: redis")
	}

	log.Infof("Initializing config system with provider: %s", providerType)

	// according to config provider type call corresponding Init method
	switch providerType {
	case "manager":
		manager.SetSystemConfigPushHandler(func(data map[string]interface{}) {
			for _, h := range managerSystemConfigHandlers {
				h(data)
			}
		})
		return manager.Init(ctx)
	case "redis":
		return redis_config.Init(ctx)
	case "memory":
		return memory.Init(ctx)
	default:
		return fmt.Errorf("unsupported config provider type: %s", providerType)
	}
}
