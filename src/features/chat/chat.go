package chat

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/layoutfull"
	"ct-go-chat/src/components/layouttabbed"
	"ct-go-chat/src/components/page"
	"ct-go-chat/src/features/chat/chatinput"
	"ct-go-chat/src/features/chat/chatstream"
	"ct-go-chat/src/features/chat/history"
	"ct-go-chat/src/features/conversation"
	"ct-go-chat/src/features/nav"
	"ct-go-chat/src/infrastructure/reqlog"
)

//go:embed chat.html
var chatHTML string

//go:embed chat.js
var chatJS string

var chatTmpl = component.WithAlpine("chat.html", chatHTML, chatJS)

var Store *conversation.Store

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /chat", handleGet)
	mux.HandleFunc("GET /chat/{conversation}", handleGetConversation)
	chatstream.RegisterRoutes(mux)
	history.Store = Store
	history.RegisterRoutes(mux)
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

func handleGetConversation(w http.ResponseWriter, r *http.Request) {
	defer reqlog.Track(r.Context(), "chat.handleGetConversation", "")()

	id := r.PathValue("conversation")
	conv, err := Store.Load(id)
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

type chatProps struct {
	ChatInputHTML    template.HTML
	ConversationJSON template.JS
}

func renderPage(conv *conversation.Conversation) (template.HTML, error) {
	chatInputHTML, err := chatinput.Render()
	if err != nil {
		return "", fmt.Errorf("chat page: render chatinput: %w", err)
	}

	convJSON := template.JS("null")
	if conv != nil {
		b, err := json.Marshal(conv)
		if err != nil {
			return "", fmt.Errorf("chat page: marshal conversation: %w", err)
		}
		convJSON = template.JS(b)
	}

	content, err := chatTmpl.Render(chatProps{
		ChatInputHTML:    chatInputHTML,
		ConversationJSON: convJSON,
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
