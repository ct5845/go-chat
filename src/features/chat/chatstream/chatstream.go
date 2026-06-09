package chatstream

import (
	"ct-go-chat/src/features/conversation"
	"ct-go-chat/src/infrastructure/llm"
	"ct-go-chat/src/infrastructure/reqlog"
	"encoding/json"
	"errors"
	"fmt"
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
	Usage          llm.Usage           `json:"usage"`
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

		conv.Messages = append(conv.Messages, llm.Message{
			Role:    "user",
			Content: req.Message,
		})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		type result struct {
			usage llm.Usage
			err   error
		}
		words := make(chan string)
		resCh := make(chan result, 1)

		go func() {
			var msgs []llm.Message
			for _, m := range conv.Messages {
				if !m.Cancelled {
					msgs = append(msgs, m)
				}
			}
			usage, err := bedrock.Respond(r.Context(), msgs, words)
			resCh <- result{usage, err}
		}()

		var assistantText strings.Builder
		for word := range words {
			assistantText.WriteString(word)
			fmt.Fprintf(w, "event: word\ndata: %s\n\n", word)
			flusher.Flush()
		}

		res := <-resCh

		assistant := llm.Message{
			Role:    "assistant",
			Content: assistantText.String(),
			Usage:   &res.usage,
		}
		if errors.Is(res.err, llm.ErrCancelled) {
			assistant.Cancelled = true
		} else if res.err != nil {
			slog.Error("chat stream error", "error", res.err)
		}
		conv.Messages = append(conv.Messages, assistant)

		if err := store.Save(conv); err != nil {
			slog.Error("chat: save conversation", "error", err)
		}

		done := doneEvent{
			ConversationID: conv.ID,
			Title:          conv.Title,
			Usage:          res.usage,
			Totals:         conv.Totals,
		}
		doneData, err := json.Marshal(done)
		if err != nil {
			slog.Error("chat: marshal done event", "error", err)
			return
		}
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
		flusher.Flush()
	}
}
