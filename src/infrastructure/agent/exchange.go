package agent

import (
	"encoding/json"
	"strings"

	"ct-go-chat/src/infrastructure/agent/bedrock"
)

// Source records where a request originated. It is a property of the
// exchange, not the conversation: a single conversation can mix origins if a
// thread started elsewhere is later continued from another surface.
type Source string

const (
	SourceWeb Source = "web"
	SourceMCP Source = "mcp"
)

// Exchange is one user request and everything it took to answer it, as a
// play-by-play of Rounds. It is the unit of persistence and UI rendering.
// Response is the joined text of all rounds — what the conversation renders
// — and Usage is the roll-up.
type Exchange struct {
	// ID is the first round's message ID; empty if the request failed
	// before the model produced anything.
	ID        string        `json:"id"`
	Request   string        `json:"request"`
	Response  string        `json:"response"`
	Source    Source        `json:"source"`
	Rounds    []Round       `json:"rounds,omitempty"`
	Usage     bedrock.Usage `json:"usage"`
	Timing    Timing        `json:"timing"`
	Cancelled bool          `json:"cancelled,omitempty"`
}

// Round is one model call plus the tool calls it requested. A non-empty
// ToolCalls means the model stopped to use tools and the next Round
// continues from their results. The raw protocol traffic is in the per-call
// log under %TEMP%/bedrock.
type Round struct {
	MessageID string        `json:"message_id"`
	Model     string        `json:"model"`
	Text      string        `json:"text,omitempty"`
	ToolCalls []ToolCall    `json:"tool_calls,omitempty"`
	Usage     bedrock.Usage `json:"usage"`
	Timing    Timing        `json:"timing"`
}

// ToolCall is one tool the model requested and what it got back.
type ToolCall struct {
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Result  string          `json:"result"`
	IsError bool            `json:"is_error,omitempty"`
}

// Timing records latency milestones in milliseconds on the exchange clock —
// measured from the start of Run, not from the start of the individual
// round.
type Timing struct {
	TTFBMs int64 `json:"ttfb_ms"` // time to first text delta
	TTLBMs int64 `json:"ttlb_ms"` // time to last text delta
}

// buildExchange derives the Exchange from the rounds collected so far. It
// works on any partial slice of rounds, so every exit path — finished,
// cancelled, error, round limit — builds its result the same way.
func buildExchange(request string, rounds []Round, cancelled bool) Exchange {
	ex := Exchange{Request: request, Rounds: rounds, Cancelled: cancelled}
	if len(rounds) > 0 {
		ex.ID = rounds[0].MessageID
	}

	var texts []string
	for _, r := range rounds {
		ex.Usage.Add(r.Usage)
		if ex.Timing.TTFBMs == 0 {
			ex.Timing.TTFBMs = r.Timing.TTFBMs
		}
		if r.Timing.TTLBMs > 0 {
			ex.Timing.TTLBMs = r.Timing.TTLBMs
		}
		if r.Text != "" {
			texts = append(texts, r.Text)
		}
	}
	ex.Response = strings.Join(texts, "\n\n")
	return ex
}
