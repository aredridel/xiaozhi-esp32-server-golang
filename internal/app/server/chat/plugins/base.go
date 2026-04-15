package plugins

import "xiaozhi-esp32-server-golang/internal/domain/chat/streamtransform"

// Init initializeoutputrelevant transform。
func Init(registry *streamtransform.Registry) {
	if registry == nil {
		return
	}

	// registeroutputbody形插件（textminute段 + tool call close）
	RegisterOutputSegmenter(registry)
}
