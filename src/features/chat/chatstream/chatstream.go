package chatstream

import (
	"ct-go-chat/src/features/conversation"
	"ct-go-chat/src/infrastructure/llm"
	"ct-go-chat/src/infrastructure/reqlog"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func RegisterRoutes(mux *http.ServeMux, store *conversation.Store, bedrock *llm.Bedrock) {
	mux.HandleFunc("POST /chat/stream", handleStream(store, bedrock))
}

type streamRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type doneEvent struct {
	ConversationID string              `json:"conversation_id"`
	Title          string              `json:"title"`
	Exchange       llm.Exchange        `json:"exchange"`
	Totals         conversation.Totals `json:"totals"`
}

func handleStream(store *conversation.Store, bedrock *llm.Bedrock) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer reqlog.Track(r.Context(), "chat.handleStream", "")()
		reqlog.IgnoreDuration(r.Context())

		var req streamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
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
				return
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		type result struct {
			exchange llm.Exchange
			err      error
		}
		events := make(chan llm.StreamEvent)
		resCh := make(chan result, 1)

		go func() {
			exchange, err := bedrock.Respond(r.Context(), conv.Exchanges, req.Message, events)
			resCh <- result{exchange, err}
		}()

		for ev := range events {
			switch ev.Type {
			case llm.StreamText:
				writeSSE(w, "word", ev.Text)
			case llm.StreamToolUse, llm.StreamToolResult:
				data, err := json.Marshal(ev.Tool)
				if err != nil {
					slog.Error("chat: marshal tool event", "error", err)
					continue
				}
				writeSSE(w, string(ev.Type), string(data))
			}
			flusher.Flush()
		}

		res := <-resCh
		if res.err != nil {
			slog.Error("chat stream error", "error", res.err)
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
		doneData, err := json.Marshal(done)
		if err != nil {
			slog.Error("chat: marshal done event", "error", err)
			return
		}
		writeSSE(w, "done", string(doneData))
		flusher.Flush()
	}
}

func writeSSE(w io.Writer, event, payload string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(payload, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
