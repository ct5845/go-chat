package chatinput

import (
	"ct-go-chat/src/components/bottomsheet"
	"ct-go-chat/src/components/component"
	_ "embed"
	"html/template"
)

//go:embed bottomsheet.html
var bottomsheetHTML string

//go:embed bottomsheet.js
var bottomsheetJS string
var bottomsheetTpl = component.WithAlpine("bottomsheet.html", bottomsheetHTML, bottomsheetJS)

func createBottomSheet(id string) (template.HTML, error) {
	content, err := bottomsheetTpl.Render(nil)
	if err != nil {
		return "", err
	}

	return bottomsheet.Render(bottomsheet.Options{
		Id:      id,
		Content: content,
		Label:   "Conversation Details",
	})
}
