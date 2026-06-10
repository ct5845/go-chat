package llm

import (
	"context"
	"encoding/json"
)

// Tool is a capability the model can invoke mid-response. InputSchema is a
// JSON Schema object describing the input; Run receives the model-provided
// input as raw JSON and returns text that is fed back to the model.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Run         func(ctx context.Context, input json.RawMessage) (string, error)
}
