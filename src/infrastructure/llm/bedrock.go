package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ct-go-chat/src/infrastructure/llm/llmprompts"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type Bedrock struct {
	client       *bedrockruntime.Client
	modelID      string
	systemPrompt string
}

func NewBedrock(region, modelID string) (*Bedrock, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("bedrock: load AWS config: %w", err)
	}
	systemPrompt, err := llmprompts.GetSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("bedrock: render system prompt: %w", err)
	}
	return &Bedrock{
		client:       bedrockruntime.NewFromConfig(cfg),
		modelID:      modelID,
		systemPrompt: systemPrompt,
	}, nil
}

func (b *Bedrock) Respond(ctx context.Context, messages []Message, chunks chan<- string) (Usage, error) {
	defer close(chunks)

	invokeTime := time.Now()

	body, err := buildRequestBody(b.systemPrompt, messages)
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

	res, err := consumeStream(ctx, stream.Events(), invokeTime, chunks)
	if err != nil {
		return Usage{}, err
	}

	if res.messageID != "" {
		writeChatLog(invokeTime, res.messageID, res.model, lastUserMessage(messages), res.usage, res.events)
	}

	usage := Usage{
		MessageID:                res.messageID,
		InputTokens:              res.usage.InputTokens,
		CacheCreationInputTokens: res.usage.CacheCreationInputTokens,
		CacheReadInputTokens:     res.usage.CacheReadInputTokens,
		OutputTokens:             res.usage.OutputTokens,
		CostUSD:                  estimateCost(res.model, res.usage, 0),
		Timing:                   Timing{TTFBMs: res.ttfbMs, TTLBMs: res.ttlbMs},
	}

	if res.cancelled {
		return usage, ErrCancelled
	}
	return usage, stream.Err()
}

// streamResult accumulates everything observed while consuming a response
// stream: identity and usage metadata, the raw events for the chat log,
// latency milestones, and whether the caller cancelled mid-stream.
type streamResult struct {
	messageID string
	model     string
	usage     tokenUsage
	events    []json.RawMessage
	cancelled bool
	ttfbMs    int64
	ttlbMs    int64
}

func consumeStream(ctx context.Context, events <-chan types.ResponseStream, invokeTime time.Time, chunks chan<- string) (streamResult, error) {
	var res streamResult
	for event := range events {
		chunk, ok := event.(*types.ResponseStreamMemberChunk)
		if !ok {
			continue
		}
		raw := chunk.Value.Bytes
		res.events = append(res.events, json.RawMessage(raw))
		parseStreamMeta(raw, &res)
		text, err := extractTextDelta(raw)
		if err != nil {
			return res, fmt.Errorf("bedrock: parse event: %w", err)
		}
		if text == "" {
			continue
		}
		elapsed := time.Since(invokeTime).Milliseconds()
		if res.ttfbMs == 0 {
			res.ttfbMs = elapsed
		}
		res.ttlbMs = elapsed
		select {
		case <-ctx.Done():
			res.cancelled = true
			return res, nil
		case chunks <- text:
		}
	}
	return res, nil
}

func lastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func buildRequestBody(systemPrompt string, messages []Message) ([]byte, error) {
	type cacheControl struct {
		Type string `json:"type"`
	}
	type content struct {
		Type         string        `json:"type"`
		Text         string        `json:"text"`
		CacheControl *cacheControl `json:"cache_control,omitempty"`
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
		System:           systemPrompt,
	}
	for _, m := range messages {
		req.Messages = append(req.Messages, bedrockMessage{
			Role:    m.Role,
			Content: []content{{Type: "text", Text: m.Content}},
		})
	}

	// Cache breakpoint on the latest message: each request reads the prefix
	// cached by the previous turn and extends it. Below the model's minimum
	// cacheable prefix this is a silent no-op, so short conversations are
	// unaffected.
	if len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1].Content
		last[len(last)-1].CacheControl = &cacheControl{Type: "ephemeral"}
	}

	return json.Marshal(req)
}

func parseStreamMeta(raw []byte, res *streamResult) {
	var event struct {
		Type    string `json:"type"`
		Message struct {
			ID    string     `json:"id"`
			Model string     `json:"model"`
			Usage tokenUsage `json:"usage"`
		} `json:"message"`
		Usage tokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return
	}
	switch event.Type {
	case "message_start":
		res.messageID = event.Message.ID
		res.model = event.Message.Model
		mergeUsage(&res.usage, event.Message.Usage)
	case "message_delta":
		mergeUsage(&res.usage, event.Usage)
	}
}

// mergeUsage overlays non-zero fields from src onto dst. Input and cache
// token counts arrive on message_start, output tokens on message_delta —
// neither event carries the full picture.
func mergeUsage(dst *tokenUsage, src tokenUsage) {
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.CacheCreationInputTokens > 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}
	if src.CacheReadInputTokens > 0 {
		dst.CacheReadInputTokens = src.CacheReadInputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
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
