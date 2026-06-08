package chatinput

import (
	_ "embed"
	"html/template"
	"ct-go-chat/src/components/bottomsheet"
	"ct-go-chat/src/components/component"
)

//go:embed bottomsheet.html
var bottomsheetHTML string
var bottomsheetTpl = component.New("bottomsheet.html", bottomsheetHTML)

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
