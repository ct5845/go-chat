package agent

// Event is one item of live output flowing from agent to client: a text
// delta, a tool starting, or a tool finishing.
type Event struct {
	Type EventType
	Text string   // EventText only
	Tool ToolCall // EventToolUse (Result empty) and EventToolResult
}

type EventType string

const (
	EventText       EventType = "text"
	EventToolUse    EventType = "tool_use"
	EventToolResult EventType = "tool_result"
)
