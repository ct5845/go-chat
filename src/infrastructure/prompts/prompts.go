// Package prompts holds the prompt text shipped with the app. The agent
// receives prompts as plain strings and never imports this package.
package prompts

import _ "embed"

//go:embed identity.txt
var system string

// System returns the chat agent's system prompt.
func System() string {
	return system
}
