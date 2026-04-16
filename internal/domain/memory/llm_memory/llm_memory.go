package llm_memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	i_redis "xiaozhi-esp32-server-golang/internal/db/redis"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	"github.com/redis/go-redis/v9"
)

var (
	memoryInstance *Memory
	once           sync.Once
	configOnce     sync.Once
)

// Memory represents conversation memory instance
type Memory struct {
	redisClient *redis.Client
	keyPrefix   string
	sync.RWMutex
}

// Get get memory instance
func Get() *Memory {
	if memoryInstance == nil {
		once.Do(func() {
			redisInstance := i_redis.GetClient()

			memoryInstance = &Memory{
				redisClient: redisInstance,
				keyPrefix:   viper.GetString("redis.key_prefix"),
			}
		})
	}
	return memoryInstance
}

// GetWithConfig use config to get memory instance (singleton pattern)
func GetWithConfig(config map[string]interface{}) (*Memory, error) {
	var initErr error
	configOnce.Do(func() {
		// read redis relevant config from config
		redisConfig, ok := config["redis"]
		if !ok {
			initErr = fmt.Errorf("redis config not exists")
			return
		}

		redisConfigMap, ok := redisConfig.(map[string]interface{})
		if !ok {
			initErr = fmt.Errorf("redis config format error")
			return
		}

		// read key_prefix config
		var keyPrefix string
		if keyPrefixInterface, exists := redisConfigMap["key_prefix"]; exists {
			if kp, ok := keyPrefixInterface.(string); ok {
				keyPrefix = kp
			} else {
				initErr = fmt.Errorf("redis.key_prefix must be string")
				return
			}
		} else {
			keyPrefix = "xiaozhi:" // default values
		}

		// get Redis client (this still uses existing Redis client get method)
		// because Redis client initialization is complex, temporarily keep existing method
		redisClient := i_redis.GetClient()
		if redisClient == nil {
			initErr = fmt.Errorf("unable to get Redis client")
			return
		}

		// create LLM memory instance
		memoryInstance = &Memory{
			redisClient: redisClient,
			keyPrefix:   keyPrefix,
		}

		log.Log().Infof("LLM memory initialized successfully, key_prefix: %s", keyPrefix)
	})

	if initErr != nil {
		return nil, initErr
	}
	return memoryInstance, nil
}

// NewWithConfig use config to create new LLM memory instance
func NewWithConfig(config map[string]interface{}) (*Memory, error) {
	// read redis relevant config from config
	redisConfig, ok := config["redis"]
	if !ok {
		return nil, fmt.Errorf("redis config not exists")
	}

	redisConfigMap, ok := redisConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("redis config format error")
	}

	// read key_prefix config
	var keyPrefix string
	if keyPrefixInterface, exists := redisConfigMap["key_prefix"]; exists {
		if kp, ok := keyPrefixInterface.(string); ok {
			keyPrefix = kp
		} else {
			return nil, fmt.Errorf("redis.key_prefix must be string")
		}
	} else {
		keyPrefix = "xiaozhi:" // default values
	}

	// get Redis client (this still uses existing Redis client get method)
	// because Redis client initialization is complex, temporarily keep existing method
	redisClient := i_redis.GetClient()
	if redisClient == nil {
		return nil, fmt.Errorf("unable to get Redis client")
	}

	// create LLM memory instance
	llmMemory := &Memory{
		redisClient: redisClient,
		keyPrefix:   keyPrefix,
	}

	log.Log().Infof("LLM memory initialized successfully, key_prefix: %s", keyPrefix)
	return llmMemory, nil
}

// NewMemory create new memory instance (only used for test)
func NewMemory(redisClient *redis.Client) *Memory {
	return &Memory{
		redisClient: redisClient,
	}
}

// getMemoryKey generate device corresponding Redis key
func (m *Memory) getMemoryKey(deviceID string) string {
	return fmt.Sprintf("%s:llm:%s", m.keyPrefix, deviceID)
}

// getSystemPromptKey generate device corresponding system prompt Redis key
func (m *Memory) getSystemPromptKey(deviceID string) string {
	return fmt.Sprintf("%s:llm:system:%s", m.keyPrefix, deviceID)
}

// AddMessage add a new conversation message to memory
func (m *Memory) AddMessage(ctx context.Context, deviceID string, agentID string, msg schema.Message) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	key := m.getMemoryKey(deviceID)
	// use nanosecond timestamp as score
	// ZREVRANGE will return score from large to small
	score := float64(time.Now().UnixNano())

	log.Debugf("add message to memory: %s, %s", key, string(msgBytes))

	return m.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(msgBytes),
	}).Err()
}

// GetMessages get all conversation memory of device
func (m *Memory) GetMessages(ctx context.Context, deviceID string, agentID string, count int) ([]*schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return []*schema.Message{}, nil
	}

	key := m.getMemoryKey(deviceID)

	if count == 0 {
		count = 10
	}

	// use ZREVRANGE to get latest N messages
	// score (timestamp) large is at front, so need to reverse order to ensure old messages are at front
	startIndex := int64(-(count))
	results, err := m.redisClient.ZRange(ctx, key, startIndex, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get messages failed: %w", err)
	}

	// preallocate slice
	messages := make([]*schema.Message, 0)

	for i := 0; i < len(results); i++ {
		msg := schema.Message{}
		if err := json.Unmarshal([]byte(results[i]), &msg); err != nil {
			return nil, fmt.Errorf("unmarshal message failed: %w", err)
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

// GetMessagesForLLM get message format suitable for LLM
func (m *Memory) GetMessagesForLLM(ctx context.Context, deviceID string, count int) ([]*schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return []*schema.Message{}, nil
	}

	// get history messages (already sorted by time: old -> new)
	memoryMessages, err := m.GetMessages(ctx, deviceID, "", count)
	if err != nil {
		return nil, err
	}

	return memoryMessages, nil
}

// SetSystemPrompt setorupdatedeviceofsystem prompt
func (m *Memory) SetSystemPrompt(ctx context.Context, deviceID string, prompt string) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	key := m.getSystemPromptKey(deviceID)
	return m.redisClient.Set(ctx, key, prompt, 0).Err()
}

// GetSystemPrompt getdeviceofsystem prompt
func (m *Memory) GetSystemPrompt(ctx context.Context, deviceID string) (schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return schema.Message{Role: schema.System, Content: viper.GetString("system_prompt")}, nil
	}

	key := m.getSystemPromptKey(deviceID)

	result, err := m.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return schema.Message{}, nil // returnemptymessagestructure
	}
	if err != nil {
		return schema.Message{}, fmt.Errorf("get system prompt failed: %w", err)
	}

	return schema.Message{
		Role:    schema.System,
		Content: result,
	}, nil
}

// ResetMemory reset conversation memory of device (including system prompt)
func (m *Memory) ResetMemory(ctx context.Context, deviceID string) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	// delete conversation history
	historyKey := m.getMemoryKey(deviceID)
	if err := m.redisClient.Del(ctx, historyKey).Err(); err != nil {
		return fmt.Errorf("delete history failed: %w", err)
	}

	return nil
}

// GetLastNMessages get latest N messages
func (m *Memory) GetLastNMessages(ctx context.Context, deviceID string, n int64) ([]schema.Message, error) {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return []schema.Message{}, nil
	}

	key := m.getMemoryKey(deviceID)

	// get last N messages
	results, err := m.redisClient.ZRevRange(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("get last messages failed: %w", err)
	}

	messages := make([]schema.Message, 0, len(results))
	for i := len(results) - 1; i >= 0; i-- { // reverse order to keep time sequential
		var msg schema.Message
		if err := json.Unmarshal([]byte(results[i]), &msg); err != nil {
			return nil, fmt.Errorf("unmarshal message failed: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// RemoveOldMessages delete messages before specified time
func (m *Memory) RemoveOldMessages(ctx context.Context, deviceID string, before time.Time) error {
	if m.redisClient == nil {
		log.Log().Warn("redis client is nil")
		return nil
	}

	key := m.getMemoryKey(deviceID)
	score := float64(before.UnixNano())

	return m.redisClient.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%f", score)).Err()
}

// GetSummary get conversation summary
func (m *Memory) GetSummary(ctx context.Context, deviceID string) (string, error) {
	return "", nil
}

// SetSummary set conversation summary
func (m *Memory) SetSummary(ctx context.Context, deviceID string, summary string) error {
	return nil
}

// Summary perform summary
func (m *Memory) Summary(ctx context.Context, deviceID string, msgList []schema.Message) (string, error) {
	return "", nil
}

func (m *Memory) GetContext(ctx context.Context, deviceID string, agentID string, maxToken int) (string, error) {
	return "", nil
}

func (m *Memory) Search(ctx context.Context, deviceID string, query string, topK int, timeRangeDays int64) (string, error) {
	return "", nil
}
