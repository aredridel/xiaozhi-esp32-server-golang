package user_config

import (
	"fmt"

	"xiaozhi-esp32-server-golang/internal/domain/config/manager"
	userconfig_redis "xiaozhi-esp32-server-golang/internal/domain/config/redis"
	"xiaozhi-esp32-server-golang/internal/util"
)

// Config userconfigprovide者configstructure
type Config struct {
	Type       string                 `json:"type"`       // storetype: "redis", "memory", "file"
	Parameters map[string]interface{} `json:"parameters"` // storerelevantconfigparameter
}

func GetProvider(sType string) (UserConfigProvider, error) {
	config := make(map[string]interface{})
	if sType == "manager" {
		// priorityfromenvironmentvariablegetbackendaddress，ifenvironmentvariableno存atthenfromconfigget
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

// GetUserConfigProvider createuserconfigprovide者
// according to传入ofstoretypeandconfigparametercreatecorrespondingprovide者instance
// providerType: provide者type，support "redis", "memory", "file"
// config: provide者configparameter
// returnUserConfigProviderinterface，support完bodyofCRUD操as
func GetUserConfigProvider(providerType string, config map[string]interface{}) (UserConfigProvider, error) {
	if config == nil {
		config = make(map[string]interface{})
	}

	switch providerType {
	case "redis":
		// createRedisuserconfigprovide者
		provider, err := userconfig_redis.NewRedisUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("createRedisuserconfigprovide者failed: %v", err)
		}
		return provider, nil
	case "manager":
		// createafterendpointmanagesystemuserconfigprovide者
		provider, err := manager.NewManagerUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("createafterendpointmanagesystemuserconfigprovide者failed: %v", err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupportedofuserconfigprovide者: %s", providerType)
	}
}
