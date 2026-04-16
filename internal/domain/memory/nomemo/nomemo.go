package nomemo

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// NoMemoProvider empty memory provider implement
// used for when user no need memory function, all methods are empty implement
type NoMemoProvider struct{}

// Get get NoMemoProvider instance
func Get() *NoMemoProvider {
	return &NoMemoProvider{}
}

// AddMessage add a message to memory (empty implement)
func (n *NoMemoProvider) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	// empty implementation, no operation executed
	return nil
}

// GetMessages get user history message (empty implement)
func (n *NoMemoProvider) GetMessages(ctx context.Context, agentId string, count int) ([]*schema.Message, error) {
	// return empty message list
	return []*schema.Message{}, nil
}

// GetContext get user context info (empty implement)
func (n *NoMemoProvider) GetContext(ctx context.Context, agentId string, maxToken int) (string, error) {
	// return empty string
	return "", nil
}

// Search search user memory (empty implement)
func (n *NoMemoProvider) Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error) {
	// return empty string
	return "", nil
}

// Flush refresh user memory (empty implement)
func (n *NoMemoProvider) Flush(ctx context.Context, agentId string) error {
	// empty implementation, no operation executed
	return nil
}

// ResetMemory reset user memory (empty implement)
func (n *NoMemoProvider) ResetMemory(ctx context.Context, agentId string) error {
	// empty implementation, no operation executed
	return nil
}
