// Package messages owns the chat message markup. Each message kind is a
// named template in messages.html, emitted at two points from the one
// definition: filled in with real data for server-rendered history, and
// zero-valued inside <template> tags for the client to clone while streaming.
package messages

import (
	"bytes"
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/agent/bedrock"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
)

//go:embed messages.html
var messagesHTML string

//go:embed messages.js
var messagesJS string

// The same source is parsed twice: templatesComp renders the root <template>
// block (plus script) for the client, partials executes the named message
// templates with real data for history.
var templatesComp = component.WithIIFE("messages.html", messagesHTML, messagesJS)
var partials = template.Must(template.New("messages.html").Parse(messagesHTML))

type userProps struct {
	Text string
}

type assistantProps struct {
	SegmentsHTML template.HTML
	DetailsHTML  template.HTML
	// HideToolbar drops the copy toolbar entirely (cancelled exchanges).
	HideToolbar bool
	// Final marks a finished exchange: toolbar visible, aria-live polite.
	// The zero value is the live-streaming shell the client clones.
	Final bool
}

type textProps struct {
	Text string
}

type toolProps struct {
	Name    string
	Input   string
	Output  string
	IsError bool
}

type detailsTriggerProps struct {
	PopoverTarget string
	OutputTokens  string
}

type detailsProps struct {
	ID               string
	InputTokens      string
	CacheWriteTokens string
	CacheReadTokens  string
	OutputTokens     string
	Cost             string
	TTFB             string
	TTLB             string
}

type templateSlots struct {
	User           userProps
	Assistant      assistantProps
	Text           textProps
	Tool           toolProps
	DetailsTrigger detailsTriggerProps
	Details        detailsProps
}

// RenderTemplates emits the <template> block and script the client uses to
// render messages while streaming.
func RenderTemplates() (template.HTML, error) {
	return templatesComp.Render(templateSlots{})
}

// RenderHistory renders a conversation's exchanges as message HTML. Assistant
// text is emitted as raw markdown text content; the client renders it on load.
func RenderHistory(exchanges []agent.Exchange) (template.HTML, error) {
	var out template.HTML
	for _, ex := range exchanges {
		user, err := renderPartial("message-user", userProps{Text: ex.Request})
		if err != nil {
			return "", err
		}
		assistant, err := renderAssistant(ex)
		if err != nil {
			return "", err
		}
		out += user + assistant
	}
	return out, nil
}

func renderAssistant(ex agent.Exchange) (template.HTML, error) {
	segments, err := renderSegments(ex.Rounds)
	if err != nil {
		return "", err
	}

	if ex.Cancelled {
		cancelled, err := renderPartial("message-cancelled", nil)
		if err != nil {
			return "", err
		}
		if segments == "" {
			return cancelled, nil
		}
		assistant, err := renderPartial("message-assistant", assistantProps{
			SegmentsHTML: segments,
			HideToolbar:  true,
		})
		if err != nil {
			return "", err
		}
		return assistant + cancelled, nil
	}

	details, err := renderDetails(ex)
	if err != nil {
		return "", err
	}
	return renderPartial("message-assistant", assistantProps{
		SegmentsHTML: segments,
		DetailsHTML:  details,
		Final:        true,
	})
}

func renderSegments(rounds []agent.Round) (template.HTML, error) {
	var out template.HTML
	for _, round := range rounds {
		if round.Text != "" {
			seg, err := renderPartial("message-assistant-text", textProps{Text: round.Text})
			if err != nil {
				return "", err
			}
			out += seg
		}
		for _, call := range round.ToolCalls {
			seg, err := renderPartial("message-tool", toolProps{
				Name:    call.Name,
				Input:   prettyJSON(call.Input),
				Output:  call.Result,
				IsError: call.IsError,
			})
			if err != nil {
				return "", err
			}
			out += seg
		}
	}
	return out, nil
}

func renderDetails(ex agent.Exchange) (template.HTML, error) {
	if ex.ID == "" {
		return "", nil
	}
	trigger, err := renderPartial("message-details-trigger", detailsTriggerProps{
		PopoverTarget: ex.ID,
		OutputTokens:  bedrock.DisplayInt(ex.Usage.OutputTokens) + " tok",
	})
	if err != nil {
		return "", err
	}
	details, err := renderPartial("message-details", detailsProps{
		ID:               ex.ID,
		InputTokens:      bedrock.DisplayInt(ex.Usage.InputTokens),
		CacheWriteTokens: bedrock.DisplayInt(ex.Usage.CacheCreationInputTokens),
		CacheReadTokens:  bedrock.DisplayInt(ex.Usage.CacheReadInputTokens),
		OutputTokens:     bedrock.DisplayInt(ex.Usage.OutputTokens),
		Cost:             bedrock.DisplayCost(ex.Usage.CostUSD),
		TTFB:             fmt.Sprintf("%d ms", ex.Timing.TTFBMs),
		TTLB:             fmt.Sprintf("%d ms", ex.Timing.TTLBMs),
	})
	if err != nil {
		return "", err
	}
	return trigger + details, nil
}

func renderPartial(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := partials.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("messages: render %s: %w", name, err)
	}
	return template.HTML(buf.String()), nil
}

func prettyJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
