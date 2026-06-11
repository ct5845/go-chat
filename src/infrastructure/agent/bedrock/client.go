package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// maxTokens caps each model reply. Constant by design — not a knob.
const maxTokens = 4096

type Client struct {
	sdk     *bedrockruntime.Client
	modelID string
}

func NewClient(region, modelID string) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("bedrock: load AWS config: %w", err)
	}
	return &Client{sdk: bedrockruntime.NewFromConfig(cfg), modelID: modelID}, nil
}

// Call performs one streamed model call and writes its call log.
// onText is called for each text delta as it arrives; returning false
// cancels the stream. On cancellation or stream error Call returns the
// partial Response accumulated so far.
func (c *Client) Call(ctx context.Context, req Request, onText func(delta string) bool) (Response, error) {
	callStart := time.Now()

	body, err := marshalRequest(req)
	if err != nil {
		return Response{}, fmt.Errorf("bedrock: marshal request: %w", err)
	}

	out, err := c.sdk.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(c.modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		writeCallLog(callStart, body, Response{}, nil)
		return Response{}, fmt.Errorf("bedrock: call: %w", err)
	}
	stream := out.GetStream()
	defer stream.Close()

	resp, raw, readErr := readStream(stream.Events(), onText)
	resp.Usage.CostUSD = estimateCost(resp.Model, resp.Usage)
	writeCallLog(callStart, body, resp, raw)

	if readErr != nil {
		return resp, readErr
	}
	return resp, stream.Err()
}

type cacheControl struct {
	Type string `json:"type"`
}

// blockJSON is a Block in the protocol's JSON shape. tool_use and
// tool_result reference the tool ID under different keys, which is why
// Block itself carries no JSON tags.
type blockJSON struct {
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

type messageJSON struct {
	Role    string      `json:"role"`
	Content []blockJSON `json:"content"`
}

func protocolBlock(b Block) blockJSON {
	switch b.Type {
	case BlockToolUse:
		return blockJSON{Type: b.Type, ID: b.ToolID, Name: b.ToolName, Input: b.ToolInput}
	case BlockToolResult:
		return blockJSON{Type: b.Type, ToolUseID: b.ToolID, Content: b.ToolResult, IsError: b.IsError}
	default:
		return blockJSON{Type: b.Type, Text: b.Text}
	}
}

func marshalRequest(req Request) ([]byte, error) {
	type requestJSON struct {
		AnthropicVersion string        `json:"anthropic_version"`
		MaxTokens        int           `json:"max_tokens"`
		System           string        `json:"system,omitempty"`
		Tools            []Tool        `json:"tools,omitempty"`
		Messages         []messageJSON `json:"messages"`
	}

	body := requestJSON{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        maxTokens,
		System:           req.System,
		Tools:            req.Tools,
	}
	for _, m := range req.Messages {
		msg := messageJSON{Role: m.Role, Content: make([]blockJSON, len(m.Blocks))}
		for i, b := range m.Blocks {
			msg.Content[i] = protocolBlock(b)
		}
		body.Messages = append(body.Messages, msg)
	}

	// Cache breakpoint on the latest Message: each request reads the prefix
	// cached by the previous call and extends it. Below the model's minimum
	// cacheable prefix this is a silent no-op, so short conversations are
	// unaffected.
	if len(body.Messages) > 0 {
		last := body.Messages[len(body.Messages)-1].Content
		last[len(last)-1].CacheControl = &cacheControl{Type: "ephemeral"}
	}

	return json.Marshal(body)
}
