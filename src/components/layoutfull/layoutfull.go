package layoutfull

import (
	"ct-go-chat/src/components/component"
	"ct-go-chat/src/components/page"
	"ct-go-chat/src/components/webcli"
	_ "embed"
	"html/template"
)

//go:embed layoutfull.html
var layoutHTML string
var comp = component.New("layoutfull.html", layoutHTML)

type Options struct {
	Header  template.HTML
	Content template.HTML
}

func Render(options Options) (template.HTML, error) {
	webCLI, err := webcli.Render()
	if err != nil {
		return "", err
	}
	return comp.Render(struct {
		Options
		WebCLI template.HTML
	}{options, webCLI})
}

func RenderPage(pageOptions page.Options, options Options) (template.HTML, error) {
	body, err := Render(options)

	if err != nil {
		return "", err
	}

	pageOptions.Body = body
	return page.Render(pageOptions)
}
