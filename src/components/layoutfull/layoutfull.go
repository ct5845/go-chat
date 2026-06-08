package layoutfull

import (
	_ "embed"
	"html/template"
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/page"
)

//go:embed layoutfull.html
var layoutHTML string
var comp = component.New("layoutfull.html", layoutHTML)

type Tab struct {
}

type Options struct {
	Header  template.HTML
	Content template.HTML
}

func Render(options Options) (template.HTML, error) {
	return comp.Render(options)
}

func RenderPage(pageOptions page.Options, options Options) (template.HTML, error) {
	if body, err := Render(options); err != nil {
		return "", err
	} else {
		pageOptions.Body = body
	}

	return page.Render(pageOptions)
}
