package nomemo

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// NoMemoProvider emptyof记忆provide者implement
// used forwhenusernoneed记忆functionwhenuse，allmethodareyesemptyimplement
type NoMemoProvider struct{}

// Get get NoMemoProvider instance
func Get() *NoMemoProvider {
	return &NoMemoProvider{}
}

// AddMessage adda条messageto记忆（emptyimplement）
func (n *NoMemoProvider) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	// emptyimplement，noexecute任何操as
	return nil
}

// GetMessages getuserofhistorymessage（emptyimplement）
func (n *NoMemoProvider) GetMessages(ctx context.Context, agentId string, count int) ([]*schema.Message, error) {
	// returnemptyofmessagelist
	return []*schema.Message{}, nil
}

// GetContext getuserofcontextinfo（emptyimplement）
func (n *NoMemoProvider) GetContext(ctx context.Context, agentId string, maxToken int) (string, error) {
	// returnemptycharstring
	return "", nil
}

// Search searchuserof记忆（emptyimplement）
func (n *NoMemoProvider) Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error) {
	// returnemptycharstring
	return "", nil
}

// Flush refreshuserof记忆（emptyimplement）
func (n *NoMemoProvider) Flush(ctx context.Context, agentId string) error {
	// emptyimplement，noexecute任何操as
	return nil
}

// ResetMemory resetuserof记忆（emptyimplement）
func (n *NoMemoProvider) ResetMemory(ctx context.Context, agentId string) error {
	// emptyimplement，noexecute任何操as
	return nil
}