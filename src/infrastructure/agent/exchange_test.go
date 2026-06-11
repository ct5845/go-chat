package agent

import (
	"encoding/json"
	"testing"

	"ct-go-chat/src/infrastructure/agent/bedrock"
)

func TestBuildExchange(t *testing.T) {
	toolRound := Round{
		MessageID: "msg_1",
		Model:     "claude-haiku-4-5",
		Text:      "Let me roll that.",
		ToolCalls: []ToolCall{{Name: "roll_dice", Input: json.RawMessage(`{}`), Result: "Rolled a d6: 4"}},
		Usage:     bedrock.Usage{InputTokens: 100, OutputTokens: 20, CostUSD: 0.001},
		Timing:    Timing{TTFBMs: 200, TTLBMs: 400},
	}
	finalRound := Round{
		MessageID: "msg_2",
		Model:     "claude-haiku-4-5",
		Text:      "You rolled a 4.",
		Usage:     bedrock.Usage{InputTokens: 150, OutputTokens: 10, CostUSD: 0.002},
		Timing:    Timing{TTFBMs: 900, TTLBMs: 1100},
	}

	tests := []struct {
		name      string
		rounds    []Round
		cancelled bool
		want      Exchange
	}{
		{
			name:   "complete",
			rounds: []Round{toolRound, finalRound},
			want: Exchange{
				ID:       "msg_1",
				Request:  "roll a die",
				Response: "Let me roll that.\n\nYou rolled a 4.",
				Rounds:   []Round{toolRound, finalRound},
				Usage:    bedrock.Usage{InputTokens: 250, OutputTokens: 30, CostUSD: 0.003},
				Timing:   Timing{TTFBMs: 200, TTLBMs: 1100},
			},
		},
		{
			name:      "cancelled mid-tools",
			rounds:    []Round{toolRound},
			cancelled: true,
			want: Exchange{
				ID:        "msg_1",
				Request:   "roll a die",
				Response:  "Let me roll that.",
				Rounds:    []Round{toolRound},
				Usage:     bedrock.Usage{InputTokens: 100, OutputTokens: 20, CostUSD: 0.001},
				Timing:    Timing{TTFBMs: 200, TTLBMs: 400},
				Cancelled: true,
			},
		},
		{
			name:   "error after first round",
			rounds: []Round{toolRound},
			want: Exchange{
				ID:       "msg_1",
				Request:  "roll a die",
				Response: "Let me roll that.",
				Rounds:   []Round{toolRound},
				Usage:    bedrock.Usage{InputTokens: 100, OutputTokens: 20, CostUSD: 0.001},
				Timing:   Timing{TTFBMs: 200, TTLBMs: 400},
			},
		},
		{
			name:      "no rounds at all",
			rounds:    nil,
			cancelled: true,
			want: Exchange{
				Request:   "roll a die",
				Cancelled: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExchange("roll a die", tt.rounds, tt.cancelled)
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("buildExchange:\n got %s\nwant %s", gotJSON, wantJSON)
			}
		})
	}
}
