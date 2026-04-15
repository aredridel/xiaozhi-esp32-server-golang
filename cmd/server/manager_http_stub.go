//go:build !manager

package main

import log "xiaozhi-esp32-server-golang/logger"

// StartManagerHTTP is a stub implementation when manager is not enabled during compilation. Use -tags manager to enable embedded manager HTTP.
func StartManagerHTTP(configPath string) {
	log.Warn("manager is not embedded in this binary, please recompile with -tags manager to enable")
}

// StopManagerHTTP is a stub implementation when manager is not enabled during compilation.
func StopManagerHTTP() {}
