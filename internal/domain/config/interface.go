package user_config

import (
	"context"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// UserConfigProvider userconfigprovide者interface
// 这yesaextendofinterface，support更多操as，区别于原haveofUserConfiginterface
type UserConfigProvider interface {
	//auth
	//according todeviceIdandclientIdgetactivateinfo
	IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error)
	GetActivationInfo(ctx context.Context, deviceId string, clientId string) (string, string, string, int)
	VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error)

	//llm memory

	// GetUserConfig getuserconfig（兼容原haveinterface）
	GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)

	// SwitchDeviceRoleByName 按rolename（supportfuzzymatching）switchdevicerole
	SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error)

	// RestoreDeviceDefaultRole recoverydevicedefaultrole（cleardevicebindrole）
	RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error

	// get mqtt, mqtt_server, udp, ota, visionconfig
	GetSystemConfig(ctx context.Context) (string, error)

	//registerup行eventprocess function(比如deviceupdown线etc)
	NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{})
	//registerdown行eventprocess function(比如message注入etc)
	RegisterMessageEventHandler(ctx context.Context, eventType string, eventHandler types.EventHandler)
}
