package agent

import (
	"encoding/json"
	"testing"

	"ct-go-chat/src/infrastructure/agent/bedrock"
)

func TestFlattenDropsToolTurns(t *testing.T) {
	withTools := Exchange{
		ID:       "msg_1",
		Request:  "roll a die",
		Response: "Let me roll that.\n\nYou rolled a 4.",
		Rounds: []Round{
			{
				MessageID: "msg_1",
				Text:      "Let me roll that.",
				ToolCalls: []ToolCall{{Name: "roll_dice", Input: json.RawMessage(`{}`), Result: "Rolled a d6: 4"}},
			},
			{MessageID: "msg_2", Text: "You rolled a 4."},
		},
	}

	msgs := flatten([]Exchange{withTools})

	// The lossy rule: tool calls and rounds collapse into exactly two
	// text-only Messages — the request and the joined response.
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	wantUser := bedrock.Message{Role: "user", Blocks: []bedrock.Block{bedrock.Text("roll a die")}}
	wantAssistant := bedrock.Message{Role: "assistant", Blocks: []bedrock.Block{bedrock.Text("Let me roll that.\n\nYou rolled a 4.")}}
	assertMessage(t, msgs[0], wantUser)
	assertMessage(t, msgs[1], wantAssistant)
}

func TestFlattenKeepsCancelledRequestDropsResponse(t *testing.T) {
	cancelled := Exchange{
		Request:   "tell me a story",
		Response:  "Once upon a",
		Cancelled: true,
	}

	msgs := flatten([]Exchange{cancelled})

	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	assertMessage(t, msgs[0], bedrock.Message{Role: "user", Blocks: []bedrock.Block{bedrock.Text("tell me a story")}})
}

func assertMessage(t *testing.T, got, want bedrock.Message) {
	t.Helper()
	if got.Role != want.Role {
		t.Errorf("role: got %q, want %q", got.Role, want.Role)
	}
	if len(got.Blocks) != 1 || got.Blocks[0].Type != bedrock.BlockText {
		t.Fatalf("blocks: got %+v, want one text block", got.Blocks)
	}
	if got.Blocks[0].Text != want.Blocks[0].Text {
		t.Errorf("text: got %q, want %q", got.Blocks[0].Text, want.Blocks[0].Text)
	}
}
