package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestConfigManager_GetSystemConfig(t *testing.T) {
	// Create config manager
	config := map[string]interface{}{
		"backend_url": "http://192.168.208.214:8080", // Adjust according to actual backend address
	}

	manager, err := NewManagerUserConfigProvider(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get system config
	configJSON, err := manager.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get system config: %v", err)
	}

	// Validate returned JSON format
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		t.Fatalf("Failed to parse config JSON: %v", err)
	}

	// Check if expected config items are included
	expectedKeys := []string{"mqtt", "mqtt_server", "udp", "ota"}
	for _, key := range expectedKeys {
		if _, exists := configMap[key]; !exists {
			t.Errorf("Config missing expected key: %s", key)
		}
	}

	fmt.Printf("Got system config: %s\n", configJSON)
	t.Logf("Config size: %d bytes", len(configJSON))
}
