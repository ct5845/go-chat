package llm

import "encoding/json"

// Exchange is one user request and everything it took to answer it, as a
// play-by-play of Invocations. Response is the joined text of all
// invocations — what the conversation renders — and Usage is the roll-up.
type Exchange struct {
	// ID is the first invocation's message ID; empty if the request failed
	// before the model produced anything.
	ID          string       `json:"id"`
	Request     string       `json:"request"`
	Response    string       `json:"response"`
	Invocations []Invocation `json:"invocations,omitempty"`
	Usage       Usage        `json:"usage"`
	Cancelled   bool         `json:"cancelled,omitempty"`
}

// Invocation is a single model invoke — one msg_bdrk_* message ID: the text
// it produced and the tool calls it requested (with their results). A
// non-empty ToolCalls means this invoke stopped to use tools and the next
// invocation continues from their results. The raw wire events are in the
// per-invoke debug log under %TEMP%/bedrock.
type Invocation struct {
	MessageID string     `json:"message_id"`
	Model     string     `json:"model"`
	Text      string     `json:"text,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     Usage      `json:"usage"`
}

// ToolCall is one tool the model invoked and what it got back.
type ToolCall struct {
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Result  string          `json:"result"`
	IsError bool            `json:"is_error,omitempty"`
}

type Usage struct {
	InputTokens              int     `json:"input_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
	Timing                   Timing  `json:"timing"`
}

// Timing records latency milestones in milliseconds, measured from the start
// of the exchange — so invocation timings are milestones on the exchange
// clock, not durations of the individual invoke.
type Timing struct {
	TTFBMs int64 `json:"ttfb_ms"` // time to first text token
	TTLBMs int64 `json:"ttlb_ms"` // time to last text token
}

// StreamEvent is one item of live response output: a text delta, a tool call
// starting, or a tool call finishing.
type StreamEvent struct {
	Type StreamEventType
	Text string   // StreamText only
	Tool ToolCall // StreamToolUse (Result empty) and StreamToolResult
}

type StreamEventType string

const (
	StreamText       StreamEventType = "text"
	StreamToolUse    StreamEventType = "tool_use"
	StreamToolResult StreamEventType = "tool_result"
)
