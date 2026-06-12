package messages

import (
	"encoding/json"
	"strings"
	"testing"

	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/agent/bedrock"
)

func TestRenderTemplates(t *testing.T) {
	out, err := RenderTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"message-user", "message-assistant", "message-assistant-text", "message-tool", "message-details-trigger", "message-details", "message-cancelled"} {
		if !strings.Contains(string(out), `<template id="`+id+`">`) {
			t.Errorf("missing template %s", id)
		}
	}
	if strings.Contains(string(out), `popovertarget=`) {
		t.Error("zero-value trigger should have no popovertarget")
	}
	if !strings.Contains(string(out), "opacity-0") {
		t.Error("zero-value assistant shell should have hidden toolbar")
	}
	if !strings.Contains(string(out), `aria-live="off"`) {
		t.Error("zero-value assistant shell should be aria-live off")
	}
}

func TestRenderHistory(t *testing.T) {
	exchanges := []agent.Exchange{
		{
			ID:       "msg-1",
			Request:  "hello <world>",
			Response: "# Hi\n\nthere",
			Rounds: []agent.Round{
				{Text: "# Hi", ToolCalls: []agent.ToolCall{{Name: "ping", Input: json.RawMessage(`{"a":1}`), Result: "pong", IsError: true}}},
				{Text: "there"},
			},
			Usage:  bedrock.Usage{InputTokens: 1234, OutputTokens: 56, CostUSD: 0.01},
			Timing: agent.Timing{TTFBMs: 100, TTLBMs: 200},
		},
		{Request: "stop this", Rounds: []agent.Round{{Text: "partial"}}, Cancelled: true},
		{Request: "cancelled before output", Cancelled: true},
	}
	out, err := RenderHistory(exchanges)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"hello &lt;world&gt;",
		`<div class="message-text"># Hi</div>`,
		`popovertarget="msg-1"`,
		`id="msg-1"`,
		"1,234",
		"56 tok",
		"$0.010000",
		"100 ms",
		`tool-output text-error`,
		"&#34;a&#34;: 1",
		"You stopped the response",
		`aria-live="polite"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	// cancelled exchanges must not get a toolbar
	if strings.Count(s, "message-toolbar") != 1 {
		t.Errorf("expected exactly one toolbar, got %d", strings.Count(s, "message-toolbar"))
	}
}
