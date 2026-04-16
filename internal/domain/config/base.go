package user_config

import (
	"fmt"

	"xiaozhi-esp32-server-golang/internal/domain/config/manager"
	userconfig_redis "xiaozhi-esp32-server-golang/internal/domain/config/redis"
	"xiaozhi-esp32-server-golang/internal/util"
)

// Config user config provider config struct
type Config struct {
	Type       string                 `json:"type"`       // store type: "redis", "memory", "file"
	Parameters map[string]interface{} `json:"parameters"` // store relevant config parameters
}

func GetProvider(sType string) (UserConfigProvider, error) {
	config := make(map[string]interface{})
	if sType == "manager" {
		// priority from environment variable get backend address, if environment variable not exists then from config get
		backendUrl := util.GetBackendURL()
		config = map[string]interface{}{
			"backend_url": backendUrl,
			"auth_token":  util.GetManagerAuthToken(),
		}
	}

	provider, err := GetUserConfigProvider(sType, config)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// GetUserConfigProvider create user config provider
// according to passed store type and config parameters create corresponding provider instance
// providerType: provider type, support "redis", "memory", "file"
// config: provider config parameters
// return UserConfigProvider interface, support full CRUD operations
func GetUserConfigProvider(providerType string, config map[string]interface{}) (UserConfigProvider, error) {
	if config == nil {
		config = make(map[string]interface{})
	}

	switch providerType {
	case "redis":
		// create Redis user config provider
		provider, err := userconfig_redis.NewRedisUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("create Redis user config provider failed: %v", err)
		}
		return provider, nil
	case "manager":
		// create backend management system user config provider
		provider, err := manager.NewManagerUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("create backend management system user config provider failed: %v", err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported user config provider: %s", providerType)
	}
}
