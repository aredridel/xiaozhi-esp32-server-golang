package plugins

import "xiaozhi-esp32-server-golang/internal/domain/chat/streamtransform"

// Init initialize output relevant transform.
func Init(registry *streamtransform.Registry) {
	if registry == nil {
		return
	}

	// register output format plugin (text segment + tool call close)
	RegisterOutputSegmenter(registry)
}
