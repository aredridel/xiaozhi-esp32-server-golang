package types

import "context"

type EventHandler func(ctx context.Context, eventType string, eventData map[string]interface{}) (string, error)

// upstream push event main program => manager internal control
const (
	EventDeviceOnline  = "/api/device/active"   // device online
	EventDeviceOffline = "/api/device/inactive" // device offline
)

// downstream pull event manager internal control => main program
const (
	EventHandleMessageInject = "/api/device/inject_msg" // process message inject
)
