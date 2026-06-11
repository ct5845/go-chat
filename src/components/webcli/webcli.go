package webcli

import (
	"ct-go-chat/src/components/component"
	_ "embed"
	"html/template"
)

var (
	//go:embed webcli.html
	webcliHTML string
	//go:embed webcli.js
	webcliJS string

	webcliTmpl = component.WithAlpine("webcli.html", webcliHTML, webcliJS)
)

func Render() (template.HTML, error) {
	return webcliTmpl.Render(nil)
}
