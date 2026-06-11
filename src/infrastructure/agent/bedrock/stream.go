package bedrock

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// readStream accumulates a Response from the raw stream events, collecting
// every raw event for the call log. onText is called with each text delta;
// returning false stops reading and the partial Response is returned with a
// nil error — the caller decides what cancellation means.
func readStream(streamEvents <-chan types.ResponseStream, onText func(string) bool) (Response, []json.RawMessage, error) {
	var resp Response
	var raw []json.RawMessage

read:
	for event := range streamEvents {
		chunk, ok := event.(*types.ResponseStreamMemberChunk)
		if !ok {
			continue
		}
		raw = append(raw, json.RawMessage(chunk.Value.Bytes))

		var ev struct {
			Type    string `json:"type"`
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage Usage  `json:"usage"`
			} `json:"message"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Usage Usage `json:"usage"`
		}
		if err := json.Unmarshal(chunk.Value.Bytes, &ev); err != nil {
			return resp, raw, fmt.Errorf("bedrock: parse stream event: %w", err)
		}

		switch ev.Type {
		case "message_start":
			resp.ID = ev.Message.ID
			resp.Model = ev.Message.Model
			mergeUsage(&resp.Usage, ev.Message.Usage)
		case "message_delta":
			mergeUsage(&resp.Usage, ev.Usage)
			if ev.Delta.StopReason != "" {
				resp.StopReason = StopReason(ev.Delta.StopReason)
			}
		case "content_block_start":
			resp.Blocks = append(resp.Blocks, Block{
				Type:     ev.ContentBlock.Type,
				ToolID:   ev.ContentBlock.ID,
				ToolName: ev.ContentBlock.Name,
			})
		case "content_block_delta":
			if len(resp.Blocks) == 0 {
				continue
			}
			block := &resp.Blocks[len(resp.Blocks)-1]
			switch ev.Delta.Type {
			case "input_json_delta":
				block.ToolInput = append(block.ToolInput, ev.Delta.PartialJSON...)
			case "text_delta":
				if ev.Delta.Text == "" {
					continue
				}
				block.Text += ev.Delta.Text
				if !onText(ev.Delta.Text) {
					break read
				}
			}
		}
	}

	resp.Blocks = finishBlocks(resp.Blocks)
	return resp, raw, nil
}

// finishBlocks drops text blocks that received no deltas (the protocol
// rejects empty text when they are echoed back) and defaults missing tool
// input to {} so it is always valid JSON.
func finishBlocks(blocks []Block) []Block {
	finished := blocks[:0]
	for _, b := range blocks {
		if b.Type == BlockText && b.Text == "" {
			continue
		}
		if b.Type == BlockToolUse && len(b.ToolInput) == 0 {
			b.ToolInput = json.RawMessage("{}")
		}
		finished = append(finished, b)
	}
	return finished
}

// mergeUsage overlays non-zero counts from src onto dst. Input and cache
// token counts arrive on message_start, output tokens on message_delta —
// neither event alone is complete.
func mergeUsage(dst *Usage, src Usage) {
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.CacheCreationInputTokens > 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}
	if src.CacheReadInputTokens > 0 {
		dst.CacheReadInputTokens = src.CacheReadInputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
}
