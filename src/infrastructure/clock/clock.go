package clock

import "time"

const (
	ToolName        = "get_time"
	ToolDescription = "Get the current date and time in the server's local timezone. Call this whenever the user asks what the time or date is — your training data does not know the current time."
)

func Now() string {
	return time.Now().Format("Monday, 2 January 2006 at 3:04:05 PM MST")
}
