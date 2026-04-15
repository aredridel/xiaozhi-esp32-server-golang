package user_config

import (
	"context"
	"testing"
)

func TestMemoryProvider(t *testing.T) {
	ctx := context.Background()

	// creatememoryprovider
	config := map[string]interface{}{
		"max_entries": 10,
	}

	provider, err := GetUserConfigProvider("memory", config)
	if err != nil {
		t.Fatalf("creatememoryproviderfailed: %v", err)
	}
	// 注意：interfaceinnoClosemethod，sononeedcall

	userID := "test_user_123"

	// by于interfaceinnoSetUserConfigmethod，我们onlytestGetUserConfigmethod
	// testgetno存atuserofconfig（shouldreturnemptyconfig）
	retrievedConfig, err := provider.GetUserConfig(ctx, userID)
	if err != nil {
		t.Fatalf("getuserconfigfailed: %v", err)
	}

	// validatereturnofyesemptyconfig
	if retrievedConfig.Llm.Provider != "" {
		t.Errorf("期望emptyconfig，but得toLLM Provider: %s", retrievedConfig.Llm.Provider)
	}

	// testsystemconfigget
	systemConfig, err := provider.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("getsystemconfigfailed: %v", err)
	}
	_ = systemConfig // systemconfigmayisempty，这yesnormalof
}

func TestProviderAdapter(t *testing.T) {
	ctx := context.Background()

	// creatememoryprovider
	provider, err := GetUserConfigProvider("memory", map[string]interface{}{
		"max_entries": 5,
	})
	if err != nil {
		t.Fatalf("creatememoryproviderfailed: %v", err)
	}
	// 注意：interfaceinnoClosemethod，sononeedcall

	// testadaptergetconfig
	userID := "adapter_test_user"

	// useadaptergetconfig（mayisemptyconfig）
	adapter := NewUserConfigAdapter(provider)
	retrievedConfig, err := adapter.GetUserConfig(ctx, userID)
	if err != nil {
		t.Fatalf("throughadaptergetconfigfailed: %v", err)
	}

	// validateadapternormal工as（gettoconfigstructure）
	if retrievedConfig.SystemPrompt == "" {
		t.Logf("adaptergettoemptyofsystemhint，这yesnormalof")
	} else {
		t.Logf("adaptergettosystemhint: %s", retrievedConfig.SystemPrompt)
	}
}

func TestDefaultConfig(t *testing.T) {
	// testRedisdefaultconfig
	redisConfig := DefaultConfig("redis")
	if redisConfig["host"] != "localhost" {
		t.Errorf("Redisdefaulthostconfigerror，期望: localhost, actual: %v", redisConfig["host"])
	}

	// testMemorydefaultconfig
	memoryConfig := DefaultConfig("memory")
	if memoryConfig["max_entries"] != 1000 {
		t.Errorf("Memorydefaultmax_entriesconfigerror，期望: 1000, actual: %v", memoryConfig["max_entries"])
	}

	// testunsupportedoftype
	unknownConfig := DefaultConfig("unknown")
	if len(unknownConfig) != 0 {
		t.Errorf("not知type应returnemptyconfig，actual: %v", unknownConfig)
	}
}

func TestValidateConfig(t *testing.T) {
	// testvalidofRedisconfig
	validRedisConfig := map[string]interface{}{
		"host": "localhost",
		"port": 6379,
	}
	err := ValidateConfig("redis", validRedisConfig)
	if err != nil {
		t.Errorf("validRedisconfigvalidatefailed: %v", err)
	}

	// testinvalidofRedisconfig（Missinghost）
	invalidRedisConfig := map[string]interface{}{
		"port": 6379,
	}
	err = ValidateConfig("redis", invalidRedisConfig)
	if err == nil {
		t.Error("MissinghostofRedisconfigshouldvalidatefailed")
	}

	// testMemoryconfig（noneedvalidate）
	err = ValidateConfig("memory", map[string]interface{}{})
	if err != nil {
		t.Errorf("Memoryconfigvalidatefailed: %v", err)
	}
}
