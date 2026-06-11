// Package agent runs an LLM in a loop with tools. One Run answers one user
// request: it flattens the prior Exchanges into protocol Messages, then
// loops — one Round per model call — running requested tools and feeding
// their results back until the model stops asking for them.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ct-go-chat/src/infrastructure/agent/bedrock"
)

// maxRounds bounds the model-call → run-tools loop so a misbehaving model
// can't burn tokens indefinitely.
const maxRounds = 8

type Agent struct {
	client *bedrock.Client
	system string
	tools  []Tool
}

func New(client *bedrock.Client, system string, tools []Tool) *Agent {
	return &Agent{client: client, system: system, tools: tools}
}

// Run answers one user request given the prior exchanges, emitting Events
// as output arrives. It always closes events before returning. On
// cancellation it returns the partial Exchange with Cancelled set and a nil
// error. On transport error it returns the partial Exchange and the error.
func (a *Agent) Run(ctx context.Context, history []Exchange, request string, events chan<- Event) (Exchange, error) {
	defer close(events)

	exchangeStart := time.Now()
	msgs := append(flatten(history), bedrock.Message{
		Role:   "user",
		Blocks: []bedrock.Block{bedrock.Text(request)},
	})

	var rounds []Round
	for range maxRounds {
		var round Round
		onText := func(delta string) bool {
			elapsed := time.Since(exchangeStart).Milliseconds()
			if round.Timing.TTFBMs == 0 {
				round.Timing.TTFBMs = elapsed
			}
			round.Timing.TTLBMs = elapsed
			return send(ctx, events, Event{Type: EventText, Text: delta})
		}

		resp, err := a.client.Call(ctx, bedrock.Request{
			System:   a.system,
			Tools:    toolDefs(a.tools),
			Messages: msgs,
		}, onText)

		// resp.ID is empty only when the call failed before the model
		// produced a message — nothing happened, so there is no round.
		if resp.ID != "" {
			round.MessageID = resp.ID
			round.Model = resp.Model
			round.Text = joinText(resp.Blocks)
			round.Usage = resp.Usage
			rounds = append(rounds, round)
		}

		if err != nil {
			if ctx.Err() != nil {
				return buildExchange(request, rounds, true), nil
			}
			return buildExchange(request, rounds, false), err
		}
		if ctx.Err() != nil {
			return buildExchange(request, rounds, true), nil
		}
		if resp.StopReason != bedrock.StopToolUse {
			return buildExchange(request, rounds, false), nil
		}

		// Separate this round's narration from the post-tool continuation
		// so streamed (and persisted) text doesn't run together.
		if round.Text != "" {
			if !send(ctx, events, Event{Type: EventText, Text: "\n\n"}) {
				return buildExchange(request, rounds, true), nil
			}
		}

		calls, results, cancelled := a.runTools(ctx, resp.Blocks, events)
		rounds[len(rounds)-1].ToolCalls = calls
		if cancelled {
			return buildExchange(request, rounds, true), nil
		}

		// The model's blocks are echoed back as the assistant turn, then the
		// tool results follow as a user Message — the conversation stays
		// strictly user/assistant alternating.
		msgs = append(msgs,
			bedrock.Message{Role: "assistant", Blocks: resp.Blocks},
			bedrock.Message{Role: "user", Blocks: results},
		)
	}
	return buildExchange(request, rounds, false), fmt.Errorf("agent: round limit (%d) exceeded", maxRounds)
}

// runTools executes every tool_use block in order, emitting EventToolUse
// and EventToolResult around each, and builds the tool_result Blocks that
// continue the conversation. Tool errors become is_error results sent back
// to the model — never Go errors.
func (a *Agent) runTools(ctx context.Context, blocks []bedrock.Block, events chan<- Event) (calls []ToolCall, results []bedrock.Block, cancelled bool) {
	for _, block := range blocks {
		if block.Type != bedrock.BlockToolUse {
			continue
		}

		call := ToolCall{Name: block.ToolName, Input: block.ToolInput}
		if !send(ctx, events, Event{Type: EventToolUse, Tool: call}) {
			return calls, results, true
		}

		output, err := a.runTool(ctx, block.ToolName, block.ToolInput)
		if err != nil {
			call.Result = err.Error()
			call.IsError = true
		} else {
			call.Result = output
		}
		calls = append(calls, call)
		results = append(results, bedrock.ToolResult(block.ToolID, call.Result, call.IsError))

		if !send(ctx, events, Event{Type: EventToolResult, Tool: call}) {
			return calls, results, true
		}
	}
	return calls, results, false
}

func (a *Agent) runTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	for _, t := range a.tools {
		if t.Name == name {
			return t.Run(ctx, input)
		}
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// joinText concatenates the text blocks of one model response.
func joinText(blocks []bedrock.Block) string {
	var text strings.Builder
	for _, b := range blocks {
		if b.Type == bedrock.BlockText {
			text.WriteString(b.Text)
		}
	}
	return text.String()
}

// send delivers one Event, reporting false when the caller has gone.
func send(ctx context.Context, events chan<- Event, ev Event) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- ev:
		return true
	}
}
