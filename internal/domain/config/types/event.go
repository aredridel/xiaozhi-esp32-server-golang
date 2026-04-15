package types

import "context"

type EventHandler func(ctx context.Context, eventType string, eventData map[string]interface{}) (string, error)

// up行pushevent mainprogram => manageinside控
const (
	EventDeviceOnline  = "/api/device/active"   //deviceup线
	EventDeviceOffline = "/api/device/inactive" //devicedown线
)

// down行pullevent manageinside控 => mainprogram
const (
	EventHandleMessageInject = "/api/device/inject_msg" //processmessage注入
)
