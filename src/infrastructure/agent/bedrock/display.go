package bedrock

import (
	"encoding/json"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var p = message.NewPrinter(language.English)

// DisplayCost renders a cost in USD for display, e.g. "$0.012345".
func DisplayCost(costUSD float64) string {
	if costUSD < 0.000001 {
		return "<$0.000001"
	}
	return p.Sprintf("$%.6f", costUSD)
}

// DisplayInt renders an integer with thousands separators, e.g. "12,345".
func DisplayInt(value int) string {
	return p.Sprintf("%d", value)
}

type usageDisplay struct {
	InputTokens              string `json:"input_tokens"`
	CacheCreationInputTokens string `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     string `json:"cache_read_input_tokens"`
	OutputTokens             string `json:"output_tokens"`
	Cost                     string `json:"cost"`
}

// MarshalJSON emits the raw counts plus a display object of pre-formatted
// strings, so the client inserts text instead of re-implementing formatting.
func (u Usage) MarshalJSON() ([]byte, error) {
	type raw Usage
	return json.Marshal(struct {
		raw
		Display usageDisplay `json:"display"`
	}{raw(u), usageDisplay{
		InputTokens:              DisplayInt(u.InputTokens),
		CacheCreationInputTokens: DisplayInt(u.CacheCreationInputTokens),
		CacheReadInputTokens:     DisplayInt(u.CacheReadInputTokens),
		OutputTokens:             DisplayInt(u.OutputTokens),
		Cost:                     DisplayCost(u.CostUSD),
	}})
}
