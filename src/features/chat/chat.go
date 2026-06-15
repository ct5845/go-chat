package chat

import (
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/layoutfull"
	"ct-go-chat/src/components/layouttabbed"
	"ct-go-chat/src/components/page"
	"ct-go-chat/src/features/chat/chatinput"
	"ct-go-chat/src/features/chat/chatstream"
	"ct-go-chat/src/features/chat/history"
	"ct-go-chat/src/features/chat/messages"
	"ct-go-chat/src/features/nav"
	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/conversation"
	"ct-go-chat/src/infrastructure/reqlog"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
)

//go:embed chat.html
var chatHTML string

//go:embed chat.js
var chatJS string

//go:embed sse.js
var sseJS string

var chatTmpl = component.WithAlpine("chat.html", chatHTML, sseJS+"\n"+chatJS)

func RegisterRoutes(mux *http.ServeMux, store *conversation.Store, chatAgent *agent.Agent) {
	mux.HandleFunc("GET /chat", handleGet)
	mux.HandleFunc("GET /chat/{conversation}", handleGetConversation(store))
	mux.HandleFunc("DELETE /chat/{conversation}", handleDeleteConversation(store))
	chatstream.RegisterRoutes(mux, store, chatAgent)
	history.RegisterRoutes(mux, store)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	defer reqlog.Track(r.Context(), "chat.handleGet", "")()

	rendered, err := renderPage(nil)
	if err != nil {
		slog.Error("Failed to render chat page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	io.WriteString(w, string(rendered))
}

func handleGetConversation(store *conversation.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer reqlog.Track(r.Context(), "chat.handleGetConversation", "")()

		id := r.PathValue("conversation")
		conv, err := store.Load(id)
		if err != nil {
			http.Error(w, "conversation not found", http.StatusNotFound)
			return
		}

		rendered, err := renderPage(conv)
		if err != nil {
			slog.Error("Failed to render chat page", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		io.WriteString(w, string(rendered))
	}
}

func handleDeleteConversation(store *conversation.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer reqlog.Track(r.Context(), "chat.handleDeleteConversation", "")()

		id := r.PathValue("conversation")
		if err := store.Delete(id); err != nil {
			slog.Error("Failed to delete conversation", "error", err, "id", id)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type chatProps struct {
	HasConversation bool
	ChatInputHTML   template.HTML
	MessagesHTML    template.HTML
	TemplatesHTML   template.HTML
	StoreJSON       template.JS
}

func renderPage(conv *conversation.Conversation) (template.HTML, error) {
	chatInputHTML, err := chatinput.Render()
	if err != nil {
		return "", fmt.Errorf("chat page: render chatinput: %w", err)
	}

	templatesHTML, err := messages.RenderTemplates()
	if err != nil {
		return "", fmt.Errorf("chat page: render message templates: %w", err)
	}

	var messagesHTML template.HTML
	storeJSON := template.JS("null")
	if conv != nil {
		messagesHTML, err = messages.RenderHistory(conv.Exchanges)
		if err != nil {
			return "", fmt.Errorf("chat page: render history: %w", err)
		}
		b, err := json.Marshal(struct {
			ID     string              `json:"id"`
			Title  string              `json:"title"`
			Totals conversation.Totals `json:"totals"`
		}{conv.ID, conv.Title, conv.Totals})
		if err != nil {
			return "", fmt.Errorf("chat page: marshal store state: %w", err)
		}
		storeJSON = template.JS(b)
	}

	content, err := chatTmpl.Render(chatProps{
		HasConversation: conv != nil,
		ChatInputHTML:   chatInputHTML,
		MessagesHTML:    messagesHTML,
		TemplatesHTML:   templatesHTML,
		StoreJSON:       storeJSON,
	})

	if err != nil {
		return "", fmt.Errorf("chat page: render content: %w", err)
	}

	bottomTabs, err := nav.Render("chat")
	if err != nil {
		return "", fmt.Errorf("chat page: render nav: %w", err)
	}

	if conv == nil {
		return layouttabbed.RenderPage(page.Options{
			Title:           "Chat",
			MetaDescription: "Chat with an AI assistant powered by AWS Bedrock",
		}, layouttabbed.Options{
			Content:    content,
			BottomTabs: bottomTabs,
		})
	} else {
		return layoutfull.RenderPage(page.Options{
			Title: conv.Title,
		}, layoutfull.Options{
			Content: content,
		})
	}
}
