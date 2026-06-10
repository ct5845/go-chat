package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ct-go-chat/src/infrastructure/llm/llmprompts"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// maxToolRounds bounds the invoke → run tools → re-invoke loop so a
// misbehaving model can't burn tokens indefinitely.
const maxToolRounds = 8

type Bedrock struct {
	client       *bedrockruntime.Client
	modelID      string
	systemPrompt string
	tools        []Tool
}

func NewBedrock(region, modelID string, tools []Tool) (*Bedrock, error) {
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
		tools:        tools,
	}, nil
}

// Respond answers one user request given the prior exchanges, streaming
// output to events as it arrives. It returns the completed Exchange — also
// on cancellation (Cancelled set, nil error) and on error (partial data).
func (b *Bedrock) Respond(ctx context.Context, history []Exchange, request string, events chan<- StreamEvent) (Exchange, error) {
	defer close(events)

	exchangeStart := time.Now()
	ex := Exchange{Request: request}
	conv := wireMessages(history, request)

	for range maxToolRounds {
		res, err := b.invokeOnce(ctx, conv, exchangeStart, events)
		recordInvocation(&ex, res, request)
		if err != nil {
			if ctx.Err() != nil {
				ex.Cancelled = true
				return ex, nil
			}
			return ex, err
		}
		if res.cancelled {
			ex.Cancelled = true
			return ex, nil
		}
		if res.stopReason != "tool_use" {
			return ex, nil
		}

		// Separate this round's narration from the post-tool continuation so
		// the streamed (and persisted) text doesn't run together.
		if last := len(ex.Invocations) - 1; last >= 0 && ex.Invocations[last].Text != "" {
			if !send(ctx, events, StreamEvent{Type: StreamText, Text: "\n\n"}) {
				ex.Cancelled = true
				return ex, nil
			}
		}

		assistant, results, calls, cancelled := b.runTools(ctx, res.blocks, events)
		if last := len(ex.Invocations) - 1; last >= 0 {
			ex.Invocations[last].ToolCalls = calls
		}
		if cancelled {
			ex.Cancelled = true
			return ex, nil
		}
		conv = append(conv, assistant, results)
	}
	return ex, fmt.Errorf("bedrock: tool loop exceeded %d rounds", maxToolRounds)
}

func (b *Bedrock) invokeOnce(ctx context.Context, conv []bedrockMessage, exchangeStart time.Time, events chan<- StreamEvent) (streamResult, error) {
	invokedAt := time.Now()

	body, err := buildRequestBody(b.systemPrompt, b.tools, conv)
	if err != nil {
		return streamResult{}, fmt.Errorf("bedrock: build request: %w", err)
	}

	out, err := b.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(b.modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return streamResult{}, fmt.Errorf("bedrock: invoke: %w", err)
	}

	stream := out.GetStream()
	defer stream.Close()

	res, err := consumeStream(ctx, stream.Events(), exchangeStart, events)
	res.invokedAt = invokedAt
	if err != nil {
		return res, err
	}
	return res, stream.Err()
}

// recordInvocation folds one invoke's results into the exchange and writes
// the per-invoke debug log. No-op when the invoke failed before the model
// produced a message.
func recordInvocation(ex *Exchange, res streamResult, request string) {
	if res.messageID == "" {
		return
	}
	inv := Invocation{
		MessageID: res.messageID,
		Model:     res.model,
		Text:      roundText(res.blocks),
		Usage: Usage{
			InputTokens:              res.usage.InputTokens,
			CacheCreationInputTokens: res.usage.CacheCreationInputTokens,
			CacheReadInputTokens:     res.usage.CacheReadInputTokens,
			OutputTokens:             res.usage.OutputTokens,
			CostUSD:                  estimateCost(res.model, res.usage, 0),
			Timing:                   Timing{TTFBMs: res.ttfbMs, TTLBMs: res.ttlbMs},
		},
	}
	ex.Invocations = append(ex.Invocations, inv)
	if ex.ID == "" {
		ex.ID = inv.MessageID
	}
	addUsage(&ex.Usage, inv.Usage)
	if inv.Text != "" {
		if ex.Response != "" {
			ex.Response += "\n\n"
		}
		ex.Response += inv.Text
	}
	writeChatLog(res.invokedAt, inv.MessageID, inv.Model, request, res.usage, inv.Usage.CostUSD, res.events)
}

func addUsage(dst *Usage, src Usage) {
	dst.InputTokens += src.InputTokens
	dst.CacheCreationInputTokens += src.CacheCreationInputTokens
	dst.CacheReadInputTokens += src.CacheReadInputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CostUSD += src.CostUSD
	if dst.Timing.TTFBMs == 0 {
		dst.Timing.TTFBMs = src.Timing.TTFBMs
	}
	if src.Timing.TTLBMs > 0 {
		dst.Timing.TTLBMs = src.Timing.TTLBMs
	}
}

func roundText(blocks []responseBlock) string {
	var text strings.Builder
	for _, block := range blocks {
		if block.blockType == "text" {
			text.WriteString(block.text)
		}
	}
	return text.String()
}

// runTools executes every tool_use block and builds the two wire messages
// that continue the conversation: the assistant turn echoing what the model
// produced, and the user turn carrying the tool results. Tool failures go
// back to the model as is_error results so it can adapt.
func (b *Bedrock) runTools(ctx context.Context, blocks []responseBlock, events chan<- StreamEvent) (assistant, results bedrockMessage, calls []ToolCall, cancelled bool) {
	assistant = bedrockMessage{Role: "assistant"}
	results = bedrockMessage{Role: "user"}
	for _, block := range blocks {
		switch block.blockType {
		case "text":
			if block.text != "" {
				assistant.Content = append(assistant.Content, contentBlock{Type: "text", Text: block.text})
			}
		case "tool_use":
			input := block.toolInput
			if input == "" {
				input = "{}"
			}
			assistant.Content = append(assistant.Content, contentBlock{
				Type:  "tool_use",
				ID:    block.toolID,
				Name:  block.toolName,
				Input: json.RawMessage(input),
			})

			call := ToolCall{Name: block.toolName, Input: json.RawMessage(input)}
			if !send(ctx, events, StreamEvent{Type: StreamToolUse, Tool: call}) {
				return assistant, results, calls, true
			}

			output, err := b.execTool(ctx, call.Name, call.Input)
			if err != nil {
				call.Result = err.Error()
				call.IsError = true
			} else {
				call.Result = output
			}
			calls = append(calls, call)
			if !send(ctx, events, StreamEvent{Type: StreamToolResult, Tool: call}) {
				return assistant, results, calls, true
			}

			results.Content = append(results.Content, contentBlock{
				Type:      "tool_result",
				ToolUseID: block.toolID,
				Content:   call.Result,
				IsError:   call.IsError,
			})
		}
	}
	return assistant, results, calls, false
}

func (b *Bedrock) execTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	for _, t := range b.tools {
		if t.Name == name {
			return t.Run(ctx, input)
		}
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// send delivers one stream event, reporting false when the caller has gone.
func send(ctx context.Context, events chan<- StreamEvent, ev StreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- ev:
		return true
	}
}

// tokenUsage is the wire-level usage shape Bedrock reports on stream events.
type tokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// responseBlock is one content block of a model response, accumulated from
// stream deltas: text for text blocks, toolID/toolName/toolInput for tool_use.
type responseBlock struct {
	blockType string
	text      string
	toolID    string
	toolName  string
	toolInput string
}

// streamResult accumulates everything observed while consuming a response
// stream: identity and usage metadata, the raw events for the chat log,
// content blocks, latency milestones, the stop reason, and whether the
// caller cancelled mid-stream.
type streamResult struct {
	messageID  string
	model      string
	usage      tokenUsage
	events     []json.RawMessage
	blocks     []responseBlock
	stopReason string
	cancelled  bool
	invokedAt  time.Time
	ttfbMs     int64
	ttlbMs     int64
}

func consumeStream(ctx context.Context, streamEvents <-chan types.ResponseStream, exchangeStart time.Time, events chan<- StreamEvent) (streamResult, error) {
	var res streamResult
	for event := range streamEvents {
		chunk, ok := event.(*types.ResponseStreamMemberChunk)
		if !ok {
			continue
		}
		raw := chunk.Value.Bytes
		res.events = append(res.events, json.RawMessage(raw))

		var ev struct {
			Type    string `json:"type"`
			Message struct {
				ID    string     `json:"id"`
				Model string     `json:"model"`
				Usage tokenUsage `json:"usage"`
			} `json:"message"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Usage tokenUsage `json:"usage"`
		}
		if err := json.Unmarshal(raw, &ev); err != nil {
			return res, fmt.Errorf("bedrock: parse event: %w", err)
		}

		switch ev.Type {
		case "message_start":
			res.messageID = ev.Message.ID
			res.model = ev.Message.Model
			mergeUsage(&res.usage, ev.Message.Usage)
		case "message_delta":
			mergeUsage(&res.usage, ev.Usage)
			if ev.Delta.StopReason != "" {
				res.stopReason = ev.Delta.StopReason
			}
		case "content_block_start":
			res.blocks = append(res.blocks, responseBlock{
				blockType: ev.ContentBlock.Type,
				toolID:    ev.ContentBlock.ID,
				toolName:  ev.ContentBlock.Name,
			})
		case "content_block_delta":
			if len(res.blocks) == 0 {
				continue
			}
			block := &res.blocks[len(res.blocks)-1]
			switch ev.Delta.Type {
			case "input_json_delta":
				block.toolInput += ev.Delta.PartialJSON
			case "text_delta":
				if ev.Delta.Text == "" {
					continue
				}
				block.text += ev.Delta.Text
				elapsed := time.Since(exchangeStart).Milliseconds()
				if res.ttfbMs == 0 {
					res.ttfbMs = elapsed
				}
				res.ttlbMs = elapsed
				if !send(ctx, events, StreamEvent{Type: StreamText, Text: ev.Delta.Text}) {
					res.cancelled = true
					return res, nil
				}
			}
		}
	}
	return res, nil
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

type cacheControl struct {
	Type string `json:"type"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type bedrockMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// wireMessages flattens prior exchanges into the alternating user/assistant
// list the model expects. Cancelled responses are dropped — the user
// rejected them — but their requests stay.
func wireMessages(history []Exchange, request string) []bedrockMessage {
	var msgs []bedrockMessage
	for _, ex := range history {
		if ex.Request != "" {
			msgs = append(msgs, bedrockMessage{
				Role:    "user",
				Content: []contentBlock{{Type: "text", Text: ex.Request}},
			})
		}
		if ex.Response != "" && !ex.Cancelled {
			msgs = append(msgs, bedrockMessage{
				Role:    "assistant",
				Content: []contentBlock{{Type: "text", Text: ex.Response}},
			})
		}
	}
	return append(msgs, bedrockMessage{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: request}},
	})
}

func buildRequestBody(systemPrompt string, tools []Tool, messages []bedrockMessage) ([]byte, error) {
	type bedrockRequest struct {
		AnthropicVersion string           `json:"anthropic_version"`
		MaxTokens        int              `json:"max_tokens"`
		System           string           `json:"system,omitempty"`
		Tools            []toolDef        `json:"tools,omitempty"`
		Messages         []bedrockMessage `json:"messages"`
	}

	req := bedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
		System:           systemPrompt,
		Messages:         messages,
	}
	for _, t := range tools {
		req.Tools = append(req.Tools, toolDef{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}

	// Cache breakpoint on the latest message: each request reads the prefix
	// cached by the previous turn and extends it. Tool rounds re-marshal the
	// same blocks, so clear stale markers before placing the new one. Below
	// the model's minimum cacheable prefix this is a silent no-op, so short
	// conversations are unaffected.
	for _, m := range req.Messages {
		for i := range m.Content {
			m.Content[i].CacheControl = nil
		}
	}
	if len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1].Content
		last[len(last)-1].CacheControl = &cacheControl{Type: "ephemeral"}
	}

	return json.Marshal(req)
}
