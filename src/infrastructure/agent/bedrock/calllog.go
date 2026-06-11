package bedrock

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// callLog is one self-contained debug record of one model call: the full
// request body exactly as sent and every raw stream event received, in
// order. Either log of a tool exchange alone shows exactly what the model
// saw and exactly what it said.
type callLog struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp"`
	Model     string            `json:"model"`
	Usage     Usage             `json:"usage"`
	Request   json.RawMessage   `json:"request"`
	Events    []json.RawMessage `json:"events"`
}

// writeCallLog writes one call log under %TEMP%/bedrock, named by message
// ID. Logging failures are reported but never fail the call.
func writeCallLog(callStart time.Time, requestBody []byte, resp Response, events []json.RawMessage) {
	entry := callLog{
		ID:        resp.ID,
		Timestamp: callStart.UTC().Format(time.RFC3339Nano),
		Model:     resp.Model,
		Usage:     resp.Usage,
		Request:   json.RawMessage(requestBody),
		Events:    events,
	}

	dir := filepath.Join(os.TempDir(), "bedrock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("bedrock: call log mkdir", "error", err)
		return
	}

	name := entry.ID
	if name == "" {
		// The call failed before the model assigned a message ID; a
		// timestamp name keeps the log from being lost.
		name = "failed-" + callStart.UTC().Format("20060102-150405.000000000")
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		slog.Error("bedrock: call log marshal", "error", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
		slog.Error("bedrock: call log write", "error", err)
	}
}
