//go:build manager

package main

import (
	"context"
	"net/http"
	"time"

	log "xiaozhi-esp32-server-golang/logger"
	mbconfig "xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/database"
	"xiaozhi/manager/backend/router"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultManagerHTTPPort   = "9000"
	defaultManagerConfigPath = "manager.json"
)

var (
	managerHTTPServer *http.Server // Embedded manager HTTP service handle in this process, used for graceful shutdown
	managerDB         *gorm.DB     // DB used by manager, closed on exit
)

// StartManagerHTTP starts the manager HTTP service within this process (dual port). Whether to call is determined by main based on -manager-enable.
// configPath: manager configuration file path, empty uses default path
func StartManagerHTTP(configPath string) {
	if configPath == "" {
		configPath = defaultManagerConfigPath
	}
	log.Infof("Starting embedded manager HTTP service, config file: %s", configPath)

	cfg := mbconfig.LoadWithPath(configPath)
	port := cfg.Server.Port
	if port == "" {
		port = defaultManagerHTTPPort
	}
	cfg.Server.Port = port

	db := database.Init(cfg.Database)
	if db == nil {
		log.Warn("Manager database initialization failed, skipping manager HTTP startup")
		return
	}
	managerDB = db

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := router.Setup(db, cfg)

	managerHTTPServer = &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Infof("Manager HTTP service started on port: %s", port)
		if err := managerHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Manager HTTP service exited abnormally: %v", err)
		}
	}()
}

// StopManagerHTTP gracefully shuts down the embedded manager HTTP service and closes database connection
func StopManagerHTTP() {
	if managerHTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := managerHTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("Manager HTTP shutdown timeout or error: %v", err)
		}
		managerHTTPServer = nil
		log.Info("Manager HTTP service closed")
	}
	if managerDB != nil {
		database.Close(managerDB)
		managerDB = nil
	}
}
