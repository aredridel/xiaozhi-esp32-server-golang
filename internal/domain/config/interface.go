package user_config

import (
	"context"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// UserConfigProvider user config provider interface
// this is an extended interface, supports more operations, different from original UserConfig interface
type UserConfigProvider interface {
	// auth
	// according to deviceId and clientId get activation info
	IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error)
	GetActivationInfo(ctx context.Context, deviceId string, clientId string) (string, string, string, int)
	VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error)

	// llm memory

	// GetUserConfig get user config (compatible with original interface)
	GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)

	// SwitchDeviceRoleByName switch device role by role name (supports fuzzy matching)
	SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error)

	// RestoreDeviceDefaultRole restore device default role (clear device bind role)
	RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error

	// get mqtt, mqtt_server, udp, ota, vision config
	GetSystemConfig(ctx context.Context) (string, error)

	// register upstream event process function (e.g. device up/down line etc)
	NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{})
	// register downstream event process function (e.g. message injection etc)
	RegisterMessageEventHandler(ctx context.Context, eventType string, eventHandler types.EventHandler)
}
