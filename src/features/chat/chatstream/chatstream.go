package chatstream

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ct-go-chat/src/features/conversation"
	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/reqlog"
)

func RegisterRoutes(mux *http.ServeMux, store *conversation.Store, chatAgent *agent.Agent) {
	mux.HandleFunc("POST /chat/stream", handleStream(store, chatAgent))
}

func handleStream(store *conversation.Store, chatAgent *agent.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer reqlog.Track(r.Context(), "chat.handleStream", "")()
		reqlog.IgnoreDuration(r.Context())

		s, ok := setup(w, r, store, chatAgent)
		if !ok {
			return
		}
		relay(w, s.flusher, s.events)
		paperwork(w, s.flusher, store, s.conv, <-s.result)
	}
}

type streamRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type runResult struct {
	exchange agent.Exchange
	err      error
}

type stream struct {
	conv    *conversation.Conversation
	flusher http.Flusher
	events  chan agent.Event
	result  chan runResult
}

// setup decodes the request, loads or creates the conversation, prepares the
// SSE response, and starts the agent in a goroutine. A false return means
// the error has already been written to w.
func setup(w http.ResponseWriter, r *http.Request, store *conversation.Store, chatAgent *agent.Agent) (stream, bool) {
	var req streamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return stream{}, false
	}

	var conv *conversation.Conversation
	if req.ConversationID == "" {
		conv = &conversation.Conversation{
			ID:      conversation.NewID(),
			Created: time.Now(),
		}
	} else {
		var err error
		conv, err = store.Load(req.ConversationID)
		if err != nil {
			http.Error(w, "conversation not found", http.StatusNotFound)
			return stream{}, false
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return stream{}, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	s := stream{
		conv:    conv,
		flusher: flusher,
		events:  make(chan agent.Event),
		result:  make(chan runResult, 1),
	}
	go func() {
		exchange, err := chatAgent.Run(r.Context(), conv.Exchanges, req.Message, s.events)
		s.result <- runResult{exchange, err}
	}()
	return s, true
}

// relay translates between the two languages of the boundary: it ranges over
// the agent's Event channel and writes one SSE frame per Event, until the
// agent closes the channel. SSE event names match Event names exactly.
func relay(w io.Writer, flusher http.Flusher, events <-chan agent.Event) {
	for ev := range events {
		switch ev.Type {
		case agent.EventText:
			writeSSE(w, string(ev.Type), ev.Text)
		case agent.EventToolUse, agent.EventToolResult:
			data, err := json.Marshal(ev.Tool)
			if err != nil {
				slog.Error("chat: marshal tool event", "error", err)
				continue
			}
			writeSSE(w, string(ev.Type), string(data))
		}
		flusher.Flush()
	}
}

type doneEvent struct {
	ConversationID string              `json:"conversation_id"`
	Title          string              `json:"title"`
	Exchange       agent.Exchange      `json:"exchange"`
	Totals         conversation.Totals `json:"totals"`
}

// paperwork appends the finished Exchange to the conversation, saves it,
// and emits the closing done frame.
func paperwork(w io.Writer, flusher http.Flusher, store *conversation.Store, conv *conversation.Conversation, res runResult) {
	if res.err != nil {
		slog.Error("chat: agent run", "error", res.err)
	}
	conv.Exchanges = append(conv.Exchanges, res.exchange)

	if err := store.Save(conv); err != nil {
		slog.Error("chat: save conversation", "error", err)
	}

	done := doneEvent{
		ConversationID: conv.ID,
		Title:          conv.Title,
		Exchange:       res.exchange,
		Totals:         conv.Totals,
	}
	data, err := json.Marshal(done)
	if err != nil {
		slog.Error("chat: marshal done event", "error", err)
		return
	}
	writeSSE(w, "done", string(data))
	flusher.Flush()
}

func writeSSE(w io.Writer, event, payload string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(payload, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
