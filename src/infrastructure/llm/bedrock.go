package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type Bedrock struct {
	client  *bedrockruntime.Client
	modelID string
}

func NewBedrock(region, modelID string) (*Bedrock, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("bedrock: load AWS config: %w", err)
	}
	return &Bedrock{
		client:  bedrockruntime.NewFromConfig(cfg),
		modelID: modelID,
	}, nil
}

func (b *Bedrock) Respond(ctx context.Context, messages []Message, chunks chan<- string) (Usage, error) {
	defer close(chunks)

	invokeTime := time.Now()

	body, err := buildRequestBody(messages)
	if err != nil {
		return Usage{}, fmt.Errorf("bedrock: build request: %w", err)
	}

	out, err := b.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(b.modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return Usage{}, fmt.Errorf("bedrock: invoke: %w", err)
	}

	stream := out.GetStream()
	defer stream.Close()

	var messageID, model string
	var tu tokenUsage
	var events []json.RawMessage
	var cancelled bool
	var ttfbMs, ttlbMs int64

	for event := range stream.Events() {
		chunk, ok := event.(*types.ResponseStreamMemberChunk)
		if !ok {
			continue
		}
		raw := chunk.Value.Bytes
		events = append(events, json.RawMessage(raw))
		parseStreamMeta(raw, &messageID, &model, &tu)
		text, err := extractTextDelta(raw)
		if err != nil {
			return Usage{}, fmt.Errorf("bedrock: parse event: %w", err)
		}
		if text == "" {
			continue
		}
		elapsed := time.Since(invokeTime).Milliseconds()
		if ttfbMs == 0 {
			ttfbMs = elapsed
		}
		ttlbMs = elapsed
		select {
		case <-ctx.Done():
			cancelled = true
		case chunks <- text:
		}
		if cancelled {
			break
		}
	}

	if messageID != "" {
		userInput := ""
		for _, m := range messages {
			if m.Role == "user" {
				userInput = m.Content
				break
			}
		}
		writeChatLog(invokeTime, messageID, model, userInput, tu, events)
	}

	usage := Usage{
		MessageID:                messageID,
		InputTokens:              tu.InputTokens,
		CacheCreationInputTokens: tu.CacheCreationInputTokens,
		CacheReadInputTokens:     tu.CacheReadInputTokens,
		OutputTokens:             tu.OutputTokens,
		CostUSD:                  estimateCost(model, tu, 0),
		Timing:                   Timing{TTFBMs: ttfbMs, TTLBMs: ttlbMs},
	}

	if cancelled {
		return usage, ErrCancelled
	}
	return usage, stream.Err()
}

func buildRequestBody(messages []Message) ([]byte, error) {
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type bedrockMessage struct {
		Role    string    `json:"role"`
		Content []content `json:"content"`
	}
	type bedrockRequest struct {
		AnthropicVersion string           `json:"anthropic_version"`
		MaxTokens        int              `json:"max_tokens"`
		System           string           `json:"system,omitempty"`
		Messages         []bedrockMessage `json:"messages"`
	}

	req := bedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
	}
	for _, m := range messages {
		if m.Role == "system" {
			req.System = m.Content
			continue
		}
		req.Messages = append(req.Messages, bedrockMessage{
			Role:    m.Role,
			Content: []content{{Type: "text", Text: m.Content}},
		})
	}
	return json.Marshal(req)
}

func parseStreamMeta(raw []byte, messageID, model *string, usage *tokenUsage) {
	var event struct {
		Type    string `json:"type"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		} `json:"message"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			OutputTokens             int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return
	}
	switch event.Type {
	case "message_start":
		*messageID = event.Message.ID
		*model = event.Message.Model
	case "message_delta":
		usage.InputTokens = event.Usage.InputTokens
		usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
		usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
		usage.OutputTokens = event.Usage.OutputTokens
	}
}

func extractTextDelta(raw []byte) (string, error) {
	var event struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return "", err
	}
	if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
		return event.Delta.Text, nil
	}
	return "", nil
}
