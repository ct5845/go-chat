package agent

import (
	"context"
	"encoding/json"

	"ct-go-chat/src/infrastructure/agent/bedrock"
)

// Tool is a capability the model can request mid-response. InputSchema is a
// JSON Schema object describing the input; Run receives the model-provided
// input as raw JSON and returns text that is fed back to the model.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Run         func(ctx context.Context, input json.RawMessage) (string, error)
}

// toolDefs strips Tools down to the schema-only shape the protocol carries.
func toolDefs(tools []Tool) []bedrock.Tool {
	defs := make([]bedrock.Tool, len(tools))
	for i, t := range tools {
		defs[i] = bedrock.Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
	}
	return defs
}
