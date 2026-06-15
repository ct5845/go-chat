package history

import (
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/icon"
	"ct-go-chat/src/components/layoutfull"
	"ct-go-chat/src/components/page"
	"ct-go-chat/src/infrastructure/conversation"
	"ct-go-chat/src/infrastructure/reqlog"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"
)

//go:embed history.html
var historyHTML string

//go:embed history.js
var historyJS string

var historyTmpl = component.WithIIFE("history.html", historyHTML, historyJS)

func formatUpdated(t, now time.Time) string {
	diff := now.Sub(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2 Jan 2006")
	}
}

func RegisterRoutes(mux *http.ServeMux, store *conversation.Store) {
	mux.HandleFunc("GET /chat/history", handleGet(store))
}

func handleGet(store *conversation.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer reqlog.Track(r.Context(), "history.handleGet", "")()

		rendered, err := renderPage(store)
		if err != nil {
			slog.Error("Failed to render history page", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		io.WriteString(w, string(rendered))
	}
}

type summaryRow struct {
	ID                     string
	Title                  string
	Updated                string
	TotalCost              string
	TotalMessages          string
	ContextWindowUsage     int
	PercentOfContextWindow string
	ContextUsedTokens      string
	ContextWindow          string
	MCPIcon                template.HTML
}

func contextWindowUsage(t conversation.Totals) int {
	if t.ContextWindow == 0 {
		return 0
	}
	return min(100, int(math.Round(float64(t.ContextUsedTokens)/float64(t.ContextWindow)*100)))
}

func renderPage(store *conversation.Store) (template.HTML, error) {
	summaries, err := store.List()
	if err != nil {
		return "", fmt.Errorf("history page: list conversations: %w", err)
	}

	now := time.Now()
	rows := make([]summaryRow, len(summaries))
	for i, s := range summaries {
		usage := contextWindowUsage(s.Totals)
		display := s.Totals.Display()
		var mcpIcon template.HTML
		if s.IncludesMCP {
			mcpIcon = icon.SVG["mcp"]
		}
		rows[i] = summaryRow{
			ID:                     s.ID,
			Title:                  s.Title,
			Updated:                formatUpdated(s.Updated, now),
			TotalCost:              display.Cost,
			TotalMessages:          display.Messages,
			ContextWindowUsage:     usage,
			PercentOfContextWindow: fmt.Sprintf("%d%% used", usage),
			ContextUsedTokens:      display.ContextUsedTokens,
			ContextWindow:          display.ContextWindow,
			MCPIcon:                mcpIcon,
		}
	}

	content, err := historyTmpl.Render(struct{ Rows []summaryRow }{Rows: rows})
	if err != nil {
		return "", fmt.Errorf("history page: render content: %w", err)
	}

	return layoutfull.RenderPage(page.Options{
		Title: "History",
	}, layoutfull.Options{
		Content: content,
	})
}
