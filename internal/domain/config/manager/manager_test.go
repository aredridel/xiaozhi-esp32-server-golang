package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestConfigManager_GetSystemConfig(t *testing.T) {
	// createconfigmanage器
	config := map[string]interface{}{
		"backend_url": "http://192.168.208.214:8080", // according toactualbackendaddress调body
	}

	manager, err := NewManagerUserConfigProvider(config)
	if err != nil {
		t.Fatalf("createconfigmanage器failed: %v", err)
	}

	// createcontext
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// getsystemconfig
	configJSON, err := manager.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("getsystemconfigfailed: %v", err)
	}

	// validatereturnofJSONformat
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		t.Fatalf("parseconfigJSONfailed: %v", err)
	}

	// check ifinclude预期ofconfig项
	expectedKeys := []string{"mqtt", "mqtt_server", "udp", "ota"}
	for _, key := range expectedKeys {
		if _, exists := configMap[key]; !exists {
			t.Errorf("configinMissing预期ofkey: %s", key)
		}
	}

	fmt.Printf("gettoofsystemconfig: %s\n", configJSON)
	t.Logf("configsize: %d byte", len(configJSON))
}
