package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	redisdb "xiaozhi-esp32-server-golang/internal/db/redis"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"

	log "xiaozhi-esp32-server-golang/logger"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/mitchellh/hashstructure/v2"
	logrus "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

// Global variables for controlling periodic updates
var (
	configUpdateTicker *time.Ticker
	configUpdateStop   chan struct{}
	configUpdateWg     sync.WaitGroup
)

func Init(configFile string) error {
	//init config
	err := initConfig(configFile)
	if err != nil {
		fmt.Printf("initConfig err: %+v", err)
		os.Exit(1)
		return err
	}

	//init log
	initLog()

	// Initialize configuration system (including WebSocket connection)
	// Note: Do not register ApplySystemConfigToViper here separately, otherwise it will execute before main's callback, causing main to read the "current config" as the merged new config; merging should happen in main's callback after reading current and comparing.
	ctx := context.Background()
	if err := user_config.InitConfigSystem(ctx); err != nil {
		fmt.Printf("Failed to initialize configuration system: %v\n", err)
	}

	// Get configuration from API and update
	if err := updateConfigFromAPI(); err != nil {
		fmt.Printf("Failed to get configuration from API, using local config: %v\n", err)
	}

	// Start periodic configuration update
	startPeriodicConfigUpdate()

	//init vad
	initVad()

	//init redis
	initRedis()

	// memory module uses lazy loading, automatically initializes when used, no explicit initialization needed

	//init auth
	err = initAuthManager()
	if err != nil {
		fmt.Printf("initAuthManager err: %+v", err)
		os.Exit(1)
		return err
	}

	return nil
}

// startPeriodicConfigUpdate starts periodic configuration updates
func startPeriodicConfigUpdate() {
	// Get update interval from config, default 5 minutes
	updateInterval := viper.GetDuration("config_provider.update_interval")
	if updateInterval <= 0 {
		updateInterval = 30 * time.Second
	}

	// Check if periodic updates are enabled
	if !viper.GetBool("config_provider.enable_periodic_update") {
		log.Info("Periodic configuration update disabled")
		return
	}

	configUpdateStop = make(chan struct{})
	configUpdateTicker = time.NewTicker(updateInterval)

	configUpdateWg.Add(1)
	go func() {
		defer configUpdateWg.Done()
		defer configUpdateTicker.Stop()

		for {
			select {
			case <-configUpdateTicker.C:
				if err := updateConfigFromAPI(); err != nil {
					log.Warnf("Periodic configuration update failed: %v", err)
				} else {
					//log.Debug("Periodic configuration update successful")
				}
			case <-configUpdateStop:
				log.Info("Periodic configuration update stopped")
				return
			}
		}
	}()

	log.Infof("Periodic configuration update started, update interval: %v", updateInterval)
}

// StopPeriodicConfigUpdate stops periodic configuration updates
func StopPeriodicConfigUpdate() {
	if configUpdateStop != nil {
		close(configUpdateStop)
		configUpdateWg.Wait()
		logrus.Info("Periodic configuration update stopped")
	}
}

func initConfig(configFile string) error {
	viper.SetConfigFile(configFile)

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

// ApplySystemConfigToViper merges system configuration into viper, used for WebSocket pushed system_config real-time updates (callback has no return value)
func ApplySystemConfigToViper(data map[string]interface{}) {
	if err := viper.MergeConfigMap(data); err != nil {
		log.Warnf("Failed to merge pushed configuration into viper: %v", err)
		return
	}
	log.Info("System configuration merged into viper from WebSocket push")
}

// SystemConfigEqual compares two system configurations for semantic equality (using hashstructure fingerprint, independent of map key order)
func SystemConfigEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		log.Debugf("[SystemConfigEqual] Result: true (both nil)")
		return true
	}
	if a == nil || b == nil {
		log.Debugf("[SystemConfigEqual] Result: false (one is nil)")
		return false
	}
	ha, err1 := hashstructure.Hash(a, hashstructure.FormatV2, nil)
	hb, err2 := hashstructure.Hash(b, hashstructure.FormatV2, nil)
	if err1 != nil || err2 != nil {
		log.Debugf("[SystemConfigEqual] Result: false (Hash failed err1=%v err2=%v)", err1, err2)
		return false
	}
	equal := ha == hb
	log.Debugf("[SystemConfigEqual] Result: %t (ha=%d hb=%d), a: %+v, b: %+v", equal, ha, hb, a, b)
	return equal
}

// updateConfigFromAPI gets configuration from API and updates viper configuration
// Internally retries continuously until successful before returning
func updateConfigFromAPI() error {
	configProviderType := viper.GetString("config_provider.type")
	retryInterval := 10 * time.Second // Retry interval
	retryCount := 0

	for {
		// Get backend management system address from configuration file
		configProvider, err := user_config.GetProvider(configProviderType)
		if err != nil {
			retryCount++
			log.Warnf("Failed to get config provider (retry %d): %v, retrying in %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		// Create context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Get system configuration JSON string
		configJSON, err := configProvider.GetSystemConfig(ctx)
		cancel()

		if err != nil {
			retryCount++
			log.Warnf("Failed to get system configuration (retry %d): %v, retrying in %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		if configJSON == "" {
			// Configuration is empty, treat as success (service may return empty config)
			if retryCount > 0 {
				log.Infof("Configuration retrieved successfully (empty config, after %d retries)", retryCount)
			}
			return nil
		}

		// Parse JSON into map
		var configMap map[string]interface{}
		if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
			retryCount++
			log.Warnf("Failed to parse configuration JSON (retry %d): %v, retrying in %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		//log.Debugf("Load config from API: %+v", configMap)

		// Use viper.MergeConfigMap to set into viper
		if err := viper.MergeConfigMap(configMap); err != nil {
			retryCount++
			log.Warnf("Failed to merge configuration into viper (retry %d): %v, retrying in %v", retryCount, err, retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		// Success
		if retryCount > 0 {
			log.Infof("Configuration retrieved successfully (after %d retries)", retryCount)
		} else {
			log.Debug("Configuration retrieved successfully")
		}
		return nil
	}
}

func initLog() error {
	// Output to file
	binPath, _ := os.Executable()
	baseDir := filepath.Dir(binPath)
	logPath := fmt.Sprintf("%s/%s%s", baseDir, viper.GetString("log.path"), viper.GetString("log.file"))
	/* Log rotation related functions
	`WithLinkName` creates a symbolic link for the latest log
	`WithRotationTime` sets log rotation time, how often to rotate
	WithMaxAge and WithRotationCount can only set one
		`WithMaxAge` sets the maximum retention time before file cleanup
		`WithRotationCount` sets the maximum number of files to keep before cleanup
	*/
	// Below configuration rotates logs every 1 minute, keeps recent 3 minutes of log files, excess automatically cleaned.
	writer, err := rotatelogs.New(
		logPath+".%Y%m%d",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithRotationCount(uint(viper.GetInt("log.max_age"))),
		rotatelogs.WithRotationTime(time.Duration(86400)*time.Second),
	)
	if err != nil {
		fmt.Printf("init log error: %v\n", err)
		os.Exit(1)
		return err
	}

	// Determine output target based on configuration
	if viper.GetBool("log.stdout") {
		// Output to both file and stdout
		multiWriter := io.MultiWriter(writer, os.Stdout)
		logrus.SetOutput(multiWriter)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", // Time format with milliseconds
			ForceColors:     true,                      // Enable colors for stdout
		})
	} else {
		// Output to file only
		logrus.SetOutput(writer)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", // Time format with milliseconds
			ForceColors:     false,                     // Disable colors for file output
		})
	}

	// Disable default caller reporting, use custom caller field
	logrus.SetReportCaller(false)
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	logrus.SetLevel(logLevel)

	return nil
}

func initVad() error {
	log.Infof("Starting VAD module initialization...")
	vadProvider := viper.GetString("vad.provider")
	log.Infof("VAD provider: %s", vadProvider)

	// VAD uses lazy loading mode, will automatically initialize on first use through global resource pool
	log.Infof("VAD module will use lazy loading mode, automatically initializes on first use")
	return nil
}

func initRedis() error {
	// Initialize our unified Redis module
	redisConfig := &redisdb.Config{
		Host:     viper.GetString("redis.host"),
		Port:     viper.GetInt("redis.port"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	}

	err := redisdb.Init(redisConfig)
	if err != nil {
		fmt.Printf("init redis error: %v\n", err)
		return err
	}

	return nil
}

func initAuthManager() error {
	return auth.Init()
}
