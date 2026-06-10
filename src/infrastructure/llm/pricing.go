package llm

import "strings"

type modelPrice struct {
	prefix                     string
	inputPer1MTokens           float64
	outputPer1MTokens          float64
	cacheWritePer1MTokensTTL5  float64
	cacheWritePer1MTokensTTL60 float64
	cacheReadPer1MTokens       float64
	contextWindow              int
}

// Prices in USD per 1M tokens.
var modelPricing = []modelPrice{
	{"claude-opus-4-8", 5.00, 25.00, 6.25, 10.00, 0.50, 1000000},
	{"claude-haiku-4-5", 1.00, 5.00, 1.25, 2.00, 0.10, 200000},
}

func ContextWindow(modelID string) int {
	for _, p := range modelPricing {
		if strings.Contains(modelID, p.prefix) {
			return p.contextWindow
		}
	}
	return 0
}

func estimateCost(modelID string, usage tokenUsage, cacheTTL int) float64 {
	for _, p := range modelPricing {
		if strings.Contains(modelID, p.prefix) {
			var cacheWritePrice float64
			if cacheTTL == 60 {
				cacheWritePrice = p.cacheWritePer1MTokensTTL60
			} else {
				cacheWritePrice = p.cacheWritePer1MTokensTTL5
			}

			return (float64(usage.InputTokens)/1_000_000)*p.inputPer1MTokens +
				(float64(usage.OutputTokens)/1_000_000)*p.outputPer1MTokens +
				(float64(usage.CacheCreationInputTokens)/1_000_000)*cacheWritePrice +
				(float64(usage.CacheReadInputTokens)/1_000_000)*p.cacheReadPer1MTokens
		}
	}
	return 0
}
