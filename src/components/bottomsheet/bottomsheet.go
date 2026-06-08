package bottomsheet

import (
	_ "embed"
	"html/template"
	"ct-go-chat/src/components/component"
)

var (
	//go:embed bottomsheet.html
	bottomsheetHTML string
	bottomsheetTpl  = component.New("bottomsheet.html", bottomsheetHTML)
)

type Options struct {
	Id      string
	Content template.HTML
	Label   string
}

func Render(options Options) (template.HTML, error) {
	return bottomsheetTpl.Render(options)
}
