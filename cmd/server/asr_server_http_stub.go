//go:build !asr_server

package main

import log "xiaozhi-esp32-server-golang/logger"

// StartAsrServerHTTP is a stub implementation when asr_server is not enabled during compilation. Use -tags asr_server to enable embedded asr_server.
func StartAsrServerHTTP(configPath string) {
	log.Warn("asr_server is not embedded in this binary, please recompile with -tags asr_server to enable")
}

// StopAsrServerHTTP is a stub implementation when asr_server is not enabled during compilation.
func StopAsrServerHTTP() {}
