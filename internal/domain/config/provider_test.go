package user_config

import (
	"context"
	"testing"
)

func TestMemoryProvider(t *testing.T) {
	ctx := context.Background()

	// Create memory provider
	config := map[string]interface{}{
		"max_entries": 10,
	}

	provider, err := GetUserConfigProvider("memory", config)
	if err != nil {
		t.Fatalf("Create memory provider failed: %v", err)
	}
	// Note: interface has no Close method, so no need to call

	userID := "test_user_123"

	// Because interface has no SetUserConfig method, we only test GetUserConfig method
	// Test getting non-existent user config (should return empty config)
	retrievedConfig, err := provider.GetUserConfig(ctx, userID)
	if err != nil {
		t.Fatalf("getuserconfigfailed: %v", err)
	}

	// Validate returned is empty config
	if retrievedConfig.Llm.Provider != "" {
		t.Errorf("Expected empty config, but got LLM Provider: %s", retrievedConfig.Llm.Provider)
	}

	// Test system config get
	systemConfig, err := provider.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("GetSystemConfig failed: %v", err)
	}
	_ = systemConfig // System config may be empty, this is normal
}

func TestProviderAdapter(t *testing.T) {
	ctx := context.Background()

	// Create memory provider
	provider, err := GetUserConfigProvider("memory", map[string]interface{}{
		"max_entries": 5,
	})
	if err != nil {
		t.Fatalf("Create memory provider failed: %v", err)
	}
	// Note: interface has no Close method, so no need to call

	// Test adapter get config
	userID := "adapter_test_user"

	// Use adapter to get config (may be empty config)
	adapter := NewUserConfigAdapter(provider)
	retrievedConfig, err := adapter.GetUserConfig(ctx, userID)
	if err != nil {
		t.Fatalf("Get config through adapter failed: %v", err)
	}

	// Validate adapter normal work (get config structure)
	if retrievedConfig.SystemPrompt == "" {
		t.Logf("Adapter got empty system prompt, this is normal")
	} else {
		t.Logf("Adapter got system prompt: %s", retrievedConfig.SystemPrompt)
	}
}

func TestDefaultConfig(t *testing.T) {
	// Test Redis default config
	redisConfig := DefaultConfig("redis")
	if redisConfig["host"] != "localhost" {
		t.Errorf("Redis default host config error, expected: localhost, actual: %v", redisConfig["host"])
	}

	// Test Memory default config
	memoryConfig := DefaultConfig("memory")
	if memoryConfig["max_entries"] != 1000 {
		t.Errorf("Memory default max_entries config error, expected: 1000, actual: %v", memoryConfig["max_entries"])
	}

	// Test unsupported type
	unknownConfig := DefaultConfig("unknown")
	if len(unknownConfig) != 0 {
		t.Errorf("Unknown type should return empty config, actual: %v", unknownConfig)
	}
}

func TestValidateConfig(t *testing.T) {
	// Test valid Redis config
	validRedisConfig := map[string]interface{}{
		"host": "localhost",
		"port": 6379,
	}
	err := ValidateConfig("redis", validRedisConfig)
	if err != nil {
		t.Errorf("Valid Redis config validate failed: %v", err)
	}

	// Test invalid Redis config (Missing host)
	invalidRedisConfig := map[string]interface{}{
		"port": 6379,
	}
	err = ValidateConfig("redis", invalidRedisConfig)
	if err == nil {
		t.Error("Missing host Redis config should validate failed")
	}

	// Test Memory config (no need validate)
	err = ValidateConfig("memory", map[string]interface{}{})
	if err != nil {
		t.Errorf("Memory config validate failed: %v", err)
	}
}
