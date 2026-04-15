package eino_llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

func (p *EinoLLMProvider) ResponseWithVllm(ctx context.Context, file []byte, text string, mimeType string) (string, error) {
	log.Infof("[Eino-LLM] startperformVLLMrequest - MIMEType: %s, file length: %d", mimeType, len(file))

	// willgraph片file以base64encode，group装isdata url
	base64Str := base64.StdEncoding.EncodeToString(file)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)

	msg := &schema.Message{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: text,
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{
					URL: dataURL,
				},
			},
		},
	}

	dialogue := []*schema.Message{
		&schema.Message{
			Role:    schema.System,
			Content: "你yesa专业ofgraph片recognize专家，pleaseaccording tograph片inside容usein文回答userof问题。",
		},
		msg,
	}
	responseChan := p.ResponseWithContext(ctx, "", dialogue, []*schema.ToolInfo{})
	if responseChan == nil {
		log.Errorf("[Eino-VLLM] call视觉apirequestprocessfailed - responseChanisnil")
		return "", fmt.Errorf("call视觉apirequestprocessfailed - responseChanisnil")
	}

	var result bytes.Buffer
	for {
		select {
		case <-ctx.Done():
			log.Errorf("[Eino-VLLM]  context done")
			return "", nil
		case response, ok := <-responseChan:
			if !ok {
				if response != nil && response.Content != "" {
					result.WriteString(response.Content)
				}
				responseText := result.String()
				return responseText, nil
			}
			result.WriteString(response.Content)
		}
	}
}
