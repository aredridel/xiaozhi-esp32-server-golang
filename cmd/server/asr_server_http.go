//go:build asr_server

package main

import (
	"context"
	"net/http"
	"time"

	"voice_server/server"
	log "xiaozhi-esp32-server-golang/logger"
)

const (
	defaultAsrServerConfigPath = "asr_server.json"
)

var (
	asrHTTPServer *http.Server // Embedded asr_server HTTP service handle in this process, used for graceful shutdown
)

// StartAsrServerHTTP starts the asr_server HTTP service within this process (independent port). Whether to call is determined by main based on -asr-enable.
// configPath: asr_server configuration file path, empty uses default path asr_server/config.json
func StartAsrServerHTTP(configPath string) {
	if configPath == "" {
		configPath = defaultAsrServerConfigPath
	}
	log.Infof("Starting embedded asr_server HTTP service, config file: %s", configPath)

	handler, addr, readTimeout, err := server.Setup(configPath)
	if err != nil {
		log.Warnf("asr_server initialization failed, skipping startup: %v", err)
		return
	}

	asrHTTPServer = &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: readTimeout,
	}

	go func() {
		log.Infof("asr_server HTTP service started on %s", addr)
		if err := asrHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("asr_server HTTP service exited abnormally: %v", err)
		}
	}()
}

// StopAsrServerHTTP gracefully shuts down the embedded asr_server HTTP service
func StopAsrServerHTTP() {
	if asrHTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := asrHTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("asr_server HTTP shutdown timeout or error: %v", err)
		}
		asrHTTPServer = nil
		log.Info("asr_server HTTP service closed")
	}
}
