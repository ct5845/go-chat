package llm

import (
	"errors"
)

var ErrCancelled = errors.New("response cancelled by user")

type Message struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"` // canonical readable text

	// assistant-only; zero values on user messages
	Usage     *Usage `json:"usage,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

type Usage struct {
	MessageID                string  `json:"message_id,omitempty"`
	InputTokens              int     `json:"input_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
	Timing                   Timing  `json:"timing"`
}

// Timing records latency milestones for a single model response.
// All durations are measured from the moment InvokeModel is called and
// expressed in milliseconds.
type Timing struct {
	TTFBMs int64 `json:"ttfb_ms"` // time to first text token
	TTLBMs int64 `json:"ttlb_ms"` // time to last text token (stream end)
}
