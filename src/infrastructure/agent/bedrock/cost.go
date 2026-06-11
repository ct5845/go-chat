package bedrock

import "strings"

// modelPrice rows are matched by substring against the model ID, so regional
// prefixes like "us.anthropic." don't matter. Prices are USD per 1M tokens;
// cache writes are priced at the 5-minute TTL, the only TTL we use.
type modelPrice struct {
	prefix                string
	inputPer1MTokens      float64
	outputPer1MTokens     float64
	cacheWritePer1MTokens float64
	cacheReadPer1MTokens  float64
	contextWindow         int
}

var modelPricing = []modelPrice{
	{"claude-opus-4-8", 5.00, 25.00, 6.25, 0.50, 1000000},
	{"claude-haiku-4-5", 1.00, 5.00, 1.25, 0.10, 200000},
}

// ContextWindow reports the model's context window in tokens, 0 if unknown.
func ContextWindow(modelID string) int {
	for _, p := range modelPricing {
		if strings.Contains(modelID, p.prefix) {
			return p.contextWindow
		}
	}
	return 0
}

// estimateCost prices one call's usage, 0 if the model is unknown.
func estimateCost(modelID string, usage Usage) float64 {
	for _, p := range modelPricing {
		if strings.Contains(modelID, p.prefix) {
			return (float64(usage.InputTokens)/1_000_000)*p.inputPer1MTokens +
				(float64(usage.OutputTokens)/1_000_000)*p.outputPer1MTokens +
				(float64(usage.CacheCreationInputTokens)/1_000_000)*p.cacheWritePer1MTokens +
				(float64(usage.CacheReadInputTokens)/1_000_000)*p.cacheReadPer1MTokens
		}
	}
	return 0
}
