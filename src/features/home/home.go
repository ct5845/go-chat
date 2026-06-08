package home

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/layouttabbed"
	"ct-go-chat/src/components/page"
	"ct-go-chat/src/features/nav"
	"ct-go-chat/src/infrastructure/reqlog"
)

//go:embed home.html
var homeHTML string
var homeTmpl = component.New("home.html", homeHTML)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", HandleGet)
}

func HandleGet(w http.ResponseWriter, r *http.Request) {
	defer reqlog.Track(r.Context(), "home.HandleGet", "")()
	page, err := render()

	if err != nil {
		slog.Error("Failed to render home page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	io.WriteString(w, string(page))
}

func render() (template.HTML, error) {
	content, err := homeTmpl.Render(map[string]any{
		"Title":       "Skopeo",
		"Description": "Survey your web presence, compare and contrast your online footprint, and gain insights into your digital identity.",
	})
	if err != nil {
		return "", fmt.Errorf("home page: render content: %w", err)
	}

	bottomTabs, err := nav.Render("home")
	if err != nil {
		return "", fmt.Errorf("home page: render bottom tabs: %w", err)
	}

	return layouttabbed.RenderPage(page.Options{
		Title:           "Skopeo",
		MetaDescription: "Survey your web presence, compare and contrast your online footprint, and gain insights into your digital identity.",
	}, layouttabbed.Options{
		Content:    content,
		BottomTabs: bottomTabs,
	})
}
