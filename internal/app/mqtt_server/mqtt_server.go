package mqtt_server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"sync"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/spf13/viper"

	log "xiaozhi-esp32-server-golang/logger"
)

var (
	currentServer *mqttServer.Server
	serverMu      sync.Mutex
)

// StartMqttServer Start MQTT server (can be called again after StopMqttServer for hot reload)
func StartMqttServer() error {
	serverMu.Lock()
	defer serverMu.Unlock()
	if currentServer != nil {
		return errors.New("mqtt_server is already running, please call StopMqttServer first")
	}
	srv := mqttServer.New(&mqttServer.Options{
		InlineClient: true,
	})

	if err := srv.AddHook(&AuthHook{}, nil); err != nil {
		log.Errorf("Failed to add AuthHook: %v", err)
		return err
	}
	deviceHook := &DeviceHook{server: srv}
	if err := srv.AddHook(deviceHook, nil); err != nil {
		log.Errorf("Failed to add DeviceHook: %v", err)
		return err
	}

	if viper.GetBool("mqtt_server.tls.enable") {
		pemFile := viper.GetString("mqtt_server.tls.pem")
		keyFile := viper.GetString("mqtt_server.tls.key")
		cert, err := tls.LoadX509KeyPair(pemFile, keyFile)
		if err != nil {
			log.Errorf("Failed to load certificate: %v", err)
			return err
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ssltcp := listeners.NewTCP(listeners.Config{
			ID:        "ssl",
			Address:   fmt.Sprintf(":%d", viper.GetInt("mqtt_server.tls.port")),
			TLSConfig: tlsConfig,
		})
		if err := srv.AddListener(ssltcp); err != nil {
			return err
		}
	}

	host := viper.GetString("mqtt_server.listen_host")
	port := viper.GetInt("mqtt_server.listen_port")
	if port == 0 {
		return errors.New("mqtt_server.port configuration error, please check the configuration file")
	}
	address := fmt.Sprintf("%s:%d", host, port)
	tcp := listeners.NewTCP(listeners.Config{Type: "tcp", ID: "t1", Address: address})
	if err := srv.AddListener(tcp); err != nil {
		return err
	}

	currentServer = srv
	log.Infof("MQTT server started, listening on address %s...", address)
	go func() {
		// Serve() returns immediately after starting listener goroutines in the library, does not block, so don't clear currentServer here
		if err := srv.Serve(); err != nil {
			log.Warnf("MQTT Server Serve exited: %v", err)
		}
	}()
	return nil
}

// StopMqttServer Stop current MQTT server for hot reload to restart StartMqttServer
func StopMqttServer() error {
	log.Infof("entering StopMqttServer ")
	defer log.Infof("exiting StopMqttServer ")
	serverMu.Lock()
	defer serverMu.Unlock()
	srv := currentServer
	if srv == nil {
		return nil
	}
	// Include Close in the same critical section to avoid concurrent Stop calling Close on the same instance repeatedly.
	if err := srv.Close(); err != nil {
		log.Warnf("StopMqttServer Close error: %v", err)
		return err
	}
	currentServer = nil
	log.Info("MQTT server stopped")
	return nil
}
