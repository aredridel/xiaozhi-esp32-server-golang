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
	// managerSystemConfigHandlers receive WebSocket system_config pushwhenofcallbacklist，mainprogram可multiple timesregister（如mergeto viper、热更service）
	managerSystemConfigHandlers []func(map[string]interface{})
)

// RegisterManagerSystemConfigHandler register manager patterndownsystemconfigpushofcallback，应at InitConfigSystem 之beforecall；可multiple timescall以追加multiplecallback
func RegisterManagerSystemConfigHandler(fn func(map[string]interface{})) {
	managerSystemConfigHandlers = append(managerSystemConfigHandlers, fn)
}

// InitConfigSystem initializeconfigsystem
// according toconfig_provider.typeofvaluecallto应configpackageofInitmethod
func InitConfigSystem(ctx context.Context) error {
	// getconfigprovide者type
	providerType := viper.GetString("config_provider.type")
	if providerType == "" {
		providerType = "redis" // defaultuseredis
		log.Infof("config_provider.type not set, using default: redis")
	}

	log.Infof("Initializing config system with provider: %s", providerType)

	// according toconfigprovide者typecallcorrespondingInitmethod
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
