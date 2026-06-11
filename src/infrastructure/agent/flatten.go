package agent

import "ct-go-chat/src/infrastructure/agent/bedrock"

// flatten converts past Exchanges into protocol Messages. The conversion is
// deliberately lossy: each prior Exchange becomes one user Message (the
// request) and one text-only assistant Message (the joined response). Tool
// turns are dropped at exchange boundaries — the model sees what was said,
// not how it was produced. Cancelled exchanges keep their user request but
// drop the assistant turn, because the user rejected it.
func flatten(history []Exchange) []bedrock.Message {
	var msgs []bedrock.Message
	for _, ex := range history {
		if ex.Request != "" {
			msgs = append(msgs, bedrock.Message{
				Role:   "user",
				Blocks: []bedrock.Block{bedrock.Text(ex.Request)},
			})
		}
		if ex.Response != "" && !ex.Cancelled {
			msgs = append(msgs, bedrock.Message{
				Role:   "assistant",
				Blocks: []bedrock.Block{bedrock.Text(ex.Response)},
			})
		}
	}
	return msgs
}
