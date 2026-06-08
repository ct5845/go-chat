package history

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/layoutfull"
	"ct-go-chat/src/components/page"
	"ct-go-chat/src/features/conversation"
	"ct-go-chat/src/infrastructure/reqlog"
	"time"
)

//go:embed history.html
var historyHTML string
var historyTmpl = component.New("history.html", historyHTML)

var Store *conversation.Store

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

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /chat/history", handleGet)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	defer reqlog.Track(r.Context(), "history.handleGet", "")()

	rendered, err := renderPage()
	if err != nil {
		slog.Error("Failed to render history page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	io.WriteString(w, string(rendered))
}

type summaryRow struct {
	ID      string
	Title   string
	Updated string
}

func renderPage() (template.HTML, error) {
	summaries, err := Store.List()
	if err != nil {
		return "", fmt.Errorf("history page: list conversations: %w", err)
	}

	now := time.Now()
	rows := make([]summaryRow, len(summaries))
	for i, s := range summaries {
		rows[i] = summaryRow{
			ID:      s.ID,
			Title:   s.Title,
			Updated: formatUpdated(s.Updated, now),
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
