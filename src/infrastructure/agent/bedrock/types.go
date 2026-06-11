// Package bedrock speaks the Anthropic messages protocol over AWS Bedrock.
// One Call is one streamed model call: a Request goes out, raw stream
// events come back and are accumulated into a Response, and a call log
// capturing both is written to disk.
package bedrock

import "encoding/json"

// Message is one turn of the conversation as the model sees it: a role and
// its content Blocks. Conversations alternate strictly user/assistant.
type Message struct {
	Role   string
	Blocks []Block
}

// Block type strings, matching the protocol's content block types.
const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
)

// Block is one content block, request or response side. Only the fields for
// its Type are set; use the constructors Text, ToolUse and ToolResult.
type Block struct {
	Type       string
	Text       string          // BlockText
	ToolID     string          // BlockToolUse, BlockToolResult
	ToolName   string          // BlockToolUse
	ToolInput  json.RawMessage // BlockToolUse
	ToolResult string          // BlockToolResult
	IsError    bool            // BlockToolResult
}

// Text returns a text Block.
func Text(s string) Block {
	return Block{Type: BlockText, Text: s}
}

// ToolUse returns a tool_use Block: the model asking for a tool to run.
func ToolUse(id, name string, input json.RawMessage) Block {
	return Block{Type: BlockToolUse, ToolID: id, ToolName: name, ToolInput: input}
}

// ToolResult returns a tool_result Block: what a tool run produced, sent
// back to the model.
func ToolResult(id, content string, isError bool) Block {
	return Block{Type: BlockToolResult, ToolID: id, ToolResult: content, IsError: isError}
}

// Tool describes one tool to the model. The runnable side lives with the
// caller; the protocol only carries the schema.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Request is everything one model call needs.
type Request struct {
	System   string
	Tools    []Tool
	Messages []Message
}

// StopReason is why the model stopped producing output.
type StopReason string

const (
	StopEndTurn   StopReason = "end_turn"
	StopToolUse   StopReason = "tool_use"
	StopMaxTokens StopReason = "max_tokens"
)

// Response is one model reply — complete, or partial on cancellation/error.
type Response struct {
	ID         string
	Model      string
	StopReason StopReason
	Blocks     []Block
	Usage      Usage
}

// Usage counts tokens and cost for one model call. Pricing knowledge lives
// in this package, so cost is filled in here, not by callers.
type Usage struct {
	InputTokens              int     `json:"input_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// Add accumulates src's counts and cost into u.
func (u *Usage) Add(src Usage) {
	u.InputTokens += src.InputTokens
	u.CacheCreationInputTokens += src.CacheCreationInputTokens
	u.CacheReadInputTokens += src.CacheReadInputTokens
	u.OutputTokens += src.OutputTokens
	u.CostUSD += src.CostUSD
}
