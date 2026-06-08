package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type chatLogEntry struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp"`
	Model     string            `json:"model"`
	UserInput string            `json:"user_input"`
	Usage     tokenUsage        `json:"usage"`
	CostUSD   float64           `json:"cost_usd"`
	Events    []json.RawMessage `json:"events"`
}

func writeChatLog(invokeTime time.Time, id, model, userInput string, usage tokenUsage, events []json.RawMessage) {
	entry := chatLogEntry{
		ID:        id,
		Timestamp: invokeTime.UTC().Format(time.RFC3339Nano),
		Model:     model,
		UserInput: userInput,
		Usage:     usage,
		CostUSD:   estimateCost(model, usage, 0),
		Events:    events,
	}

	dir := filepath.Join(os.TempDir(), "bedrock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "chatlog: mkdir: %v\n", err)
		return
	}

	name := filepath.Join(dir, id+".json")
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "chatlog: marshal: %v\n", err)
		return
	}
	if err := os.WriteFile(name, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "chatlog: write: %v\n", err)
	}
}
