// Package tools holds the tool implementations the chat agent can run.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/clock"
	"ct-go-chat/src/infrastructure/dice"
)

func All() []agent.Tool {
	return []agent.Tool{rollDice(), getTime()}
}

func rollDice() agent.Tool {
	return agent.Tool{
		Name:        dice.ToolName,
		Description: dice.ToolDescription,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"sides": {"type": "integer", "description": "Number of sides per die. Defaults to 6."},
				"count": {"type": "integer", "description": "Number of dice to roll. Defaults to 1."}
			}
		}`),
		Run: func(ctx context.Context, input json.RawMessage) (string, error) {
			args := struct {
				Sides int `json:"sides"`
				Count int `json:"count"`
			}{Sides: 6, Count: 1}
			if err := json.Unmarshal(input, &args); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			return dice.Roll(args.Sides, args.Count)
		},
	}
}

func getTime() agent.Tool {
	return agent.Tool{
		Name:        clock.ToolName,
		Description: clock.ToolDescription,
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		Run: func(ctx context.Context, input json.RawMessage) (string, error) {
			return clock.Now(), nil
		},
	}
}
