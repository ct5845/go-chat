// Package tools holds the tool implementations the chat agent can run.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"ct-go-chat/src/infrastructure/agent"
)

func All() []agent.Tool {
	return []agent.Tool{rollDice(), getTime()}
}

func rollDice() agent.Tool {
	return agent.Tool{
		Name:        "roll_dice",
		Description: "Roll one or more dice and return the results. Call this whenever the user asks to roll dice or wants a random dice outcome — do not invent dice results yourself.",
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
			if args.Sides < 2 || args.Count < 1 || args.Count > 100 {
				return "", fmt.Errorf("invalid dice: sides must be >= 2 and count between 1 and 100")
			}

			rolls := make([]string, args.Count)
			total := 0
			for i := range args.Count {
				roll := rand.IntN(args.Sides) + 1
				total += roll
				rolls[i] = strconv.Itoa(roll)
			}
			if args.Count == 1 {
				return fmt.Sprintf("Rolled a d%d: %s", args.Sides, rolls[0]), nil
			}
			return fmt.Sprintf("Rolled %dd%d: %s (total %d)", args.Count, args.Sides, strings.Join(rolls, ", "), total), nil
		},
	}
}

func getTime() agent.Tool {
	return agent.Tool{
		Name:        "get_time",
		Description: "Get the current date and time in the server's local timezone. Call this whenever the user asks what the time or date is — your training data does not know the current time.",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		Run: func(ctx context.Context, input json.RawMessage) (string, error) {
			return time.Now().Format("Monday, 2 January 2006 at 3:04:05 PM MST"), nil
		},
	}
}
