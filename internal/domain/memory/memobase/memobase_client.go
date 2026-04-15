package memobase

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/memodb-io/memobase/src/client/memobase-go/blob"
	"github.com/memodb-io/memobase/src/client/memobase-go/core"
)

var (
	clientInstance *MemobaseClient
	once           sync.Once
	configOnce     sync.Once
	// use fixed namespace UUID, used for generating device ID UUID v5
	// this ensures that the same device ID always maps to the same UUID
	deviceNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // DNS namespace
)

// MemobaseClient Memobase client manager
type MemobaseClient struct {
	client *core.MemoBaseClient
	users  sync.Map // cache user objects
	sync.RWMutex
	EnableSearch    bool
	SearchThreshold float64
	SearchTopk      int
}

// GetWithConfig gets Memobase client instance using config (singleton pattern)
func GetWithConfig(config map[string]interface{}) (*MemobaseClient, error) {
	var initErr error
	configOnce.Do(func() {
		iClient := &MemobaseClient{
			users: sync.Map{},
		}
		// read memobase relevant config from config
		// read required config items
		projectUrlInterface, ok := config["base_url"]
		if !ok {
			initErr = fmt.Errorf("memobase.base_url config is missing")
			return
		}
		baseUrl, ok := projectUrlInterface.(string)
		if !ok {
			initErr = fmt.Errorf("memobase.base_url must be a string")
			return
		}

		apiKeyInterface, ok := config["api_key"]
		if !ok {
			initErr = fmt.Errorf("memobase.api_key config is missing")
			return
		}
		apiKey, ok := apiKeyInterface.(string)
		if !ok {
			initErr = fmt.Errorf("memobase.api_key must be a string")
			return
		}

		if baseUrl == "" || apiKey == "" {
			initErr = fmt.Errorf("Memobase config incomplete: base_url or api_key is empty")
			log.Log().Errorf("Memobase initialization failed: %v", initErr)
			return
		}

		// read optional search config
		enableSearchInterface, ok := config["enable_search"]
		if ok {
			enableSearch, ok := enableSearchInterface.(bool)
			if ok {
				iClient.EnableSearch = enableSearch
			}
		}

		thresholdInterface, ok := config["search_threshold"]
		if ok {
			threshold, ok := thresholdInterface.(float64)
			if ok {
				iClient.SearchThreshold = threshold
			}
		}

		topKInterface, ok := config["search_topk"]
		if ok {
			topK, ok := topKInterface.(int)
			if ok {
				iClient.SearchTopk = topK
			}
		}

		// create client
		client, err := core.NewMemoBaseClient(baseUrl, apiKey)
		if err != nil {
			initErr = fmt.Errorf("create Memobase client failed: %v", err)
			log.Log().Errorf("Memobase initialization failed: %v", initErr)
			return
		}

		iClient.client = client
		clientInstance = iClient

		log.Log().Infof("Memobase client initialized successfully, project_url: %s", baseUrl)
	})

	if initErr != nil {
		return nil, initErr
	}
	return clientInstance, nil
}

// deviceIDToUUID converts deviceID to UUID v5 format
// use UUID v5 to ensure the same deviceID always generates the same UUID
func deviceIDToUUID(deviceID string) string {
	return uuid.NewSHA1(deviceNamespace, []byte(deviceID)).String()
}

func IsEnableSearch() bool {
	return clientInstance.EnableSearch
}

// AddMessage adds message to Memobase
func (m *MemobaseClient) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	memobaseUserID := deviceIDToUUID(agentID)
	// build message
	messages := []blob.OpenAICompatibleMessage{
		{
			Role:    string(msg.Role),
			Content: msg.Content,
		},
	}

	// if have tool call, add to message
	if len(msg.ToolCalls) > 0 {
		return nil
		/*for _, toolCall := range msg.ToolCalls {
			messages = append(messages, blob.OpenAICompatibleMessage{
				Role:    "tool",
				Content: fmt.Sprintf("Tool: %s, Args: %v", toolCall.Function.Name, toolCall.Function.Arguments),
			})
		}*/
	}

	// create ChatBlob
	chatBlob := &blob.ChatBlob{
		BaseBlob: blob.BaseBlob{
			Type: blob.ChatType,
		},
		Messages: messages,
	}

	// get or create user instance (use UUID format of userID)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("get or create user failed, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return fmt.Errorf("get or create user failed: %v", err)
	}

	// insert message (async)
	blobID, err := user.Insert(chatBlob, false)
	if err != nil {
		log.Log().Errorf("add message to Memobase failed, deviceID: %s, error: %v", agentID, err)
		return fmt.Errorf("add message to Memobase failed: %v", err)
	}

	//user.Flush(blob.ChatType, false)

	log.Log().Debugf("successfully added message to Memobase, deviceID: %s, blobID: %s", agentID, blobID)
	return nil
}

func (m *MemobaseClient) Flush(ctx context.Context, agentID string) error {
	memobaseUserID := deviceIDToUUID(agentID)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("refresh user memory failed, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return fmt.Errorf("refresh user memory failed: %v", err)
	}
	user.Flush(blob.ChatType, false)
	return nil
}

// GetContext gets user context
func (m *MemobaseClient) GetContext(ctx context.Context, agentID string, maxToken int) (string, error) {

	// convert deviceID to UUID format (required by Memobase)
	memobaseUserID := deviceIDToUUID(agentID)

	// get user instance (does not execute HTTP GET request, only creates instance)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("get user instance failed, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("get user instance failed: %v", err)
	}

	// get context, use default options
	context, err := user.Context(&core.ContextOptions{
		MaxTokenSize: maxToken,
	})
	if err != nil {
		log.Log().Errorf("get context from Memobase failed, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("get context from Memobase failed: %v", err)
	}

	log.Log().Debugf("successfully got context from Memobase, agentID: %s, context length: %d", agentID, len(context))
	return context, nil
}

func (m *MemobaseClient) Search(ctx context.Context, agentID string, query string, topK int, timeRangeDays int64) (string, error) {
	if !m.EnableSearch {
		return "", nil
	}
	topK = m.SearchTopk
	// convert deviceID to UUID format (required by Memobase)
	memobaseUserID := deviceIDToUUID(agentID)

	// get user instance (does not execute HTTP GET request, only creates instance)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("get user instance failed, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("get user instance failed: %v", err)
	}

	topK = 2

	// search event
	userEventList, err := user.SearchEvent(query, topK, 0.2, int(timeRangeDays))
	if err != nil {
		log.Log().Errorf("search event from Memobase failed, agentID: %s, error: %v", agentID, err)
		return "", fmt.Errorf("search event from Memobase failed: %v", err)
	}

	var eventList []string
	for _, event := range userEventList {
		eventList = append(eventList, fmt.Sprintf("- %s: %s", event.CreatedAt, event.EventData.EventTip))
	}

	// convert to string
	userEventStr := strings.Join(eventList, "\n")

	log.Log().Debugf("successfully searched event from Memobase, agentID: %s, event count: %d", agentID, len(eventList))
	return userEventStr, nil
}

// AddBatchMessages batch adds messages to Memobase
func (m *MemobaseClient) AddBatchMessages(ctx context.Context, userID string, messages []schema.Message) error {
	m.Lock()
	defer m.Unlock()

	if len(messages) == 0 {
		return nil
	}

	// convert message format
	blobMessages := make([]blob.OpenAICompatibleMessage, 0, len(messages))
	for _, msg := range messages {
		blobMessages = append(blobMessages, blob.OpenAICompatibleMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// create ChatBlob
	chatBlob := &blob.ChatBlob{
		BaseBlob: blob.BaseBlob{
			Type: blob.ChatType,
		},
		Messages: blobMessages,
	}

	// convert deviceID to UUID format (required by Memobase)
	memobaseUserID := deviceIDToUUID(userID)

	// get or create user instance (use UUID format of userID)
	user, err := m.getUser(userID)
	if err != nil {
		log.Log().Errorf("batch add message: get or create user failed, deviceID: %s, memobaseUserID: %s, error: %v", userID, memobaseUserID, err)
		return fmt.Errorf("get or create user failed: %v", err)
	}

	// insert message (async)
	blobID, err := user.Insert(chatBlob, false)
	if err != nil {
		log.Log().Errorf("batch add message to Memobase failed, deviceID: %s, error: %v", userID, err)
		return fmt.Errorf("batch add message to Memobase failed: %v", err)
	}

	log.Log().Debugf("successfully batch added %d messages to Memobase, deviceID: %s, blobID: %s", len(messages), userID, blobID)
	return nil
}

// GetMessages gets user's history messages
// implements BaseMemoryProvider interface
// Note: Memobase is mainly used for long-term memory and context enhancement, does not provide history message retrieve function
func (m *MemobaseClient) GetMessages(ctx context.Context, agentID string, count int) ([]*schema.Message, error) {
	return []*schema.Message{}, nil
}

// ResetMemory resets user's memories
// implements MemoryProvider interface
// Note: Memobase memory reset needs to be done through API to delete user data
func (m *MemobaseClient) ResetMemory(ctx context.Context, userID string) error {
	// TODO: if Memobase SDK provides delete user data interface, call it here
	// currently return nil to indicate operation successful (even if no actual delete)
	log.Log().Infof("Memobase reset memory request: userID=%s (Note: Memobase does not support direct reset)", userID)
	return nil
}

// Close closes client (if needed)
func (m *MemobaseClient) Close() error {
	log.Log().Info("Memobase client already closed")
	return nil
}

// todo add user object cache
func (m *MemobaseClient) getUser(userID string) (*core.User, error) {
	if user, ok := m.users.Load(userID); ok {
		return user.(*core.User), nil
	}

	memobaseUserID := deviceIDToUUID(userID)
	user, err := m.client.GetOrCreateUser(memobaseUserID)
	if err != nil {
		return nil, fmt.Errorf("get user instance failed: %v", err)
	}

	m.users.Store(userID, user)
	return user, nil
}
