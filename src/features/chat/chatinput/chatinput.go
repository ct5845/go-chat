package chatinput

import (
	"ct-go-chat/src/components/component"
	_ "embed"
	"html/template"
)

//go:embed chatinput.html
var chatInputHTML string

//go:embed chatinput.js
var chatInputJS string

var chatInputTmpl = component.WithAlpine("chatinput.html", chatInputHTML, chatInputJS)

func Render() (template.HTML, error) {
	bottomsheetId := "chat-bottomsheet"
	bottomsheet, err := createBottomSheet(bottomsheetId)
	if err != nil {
		return "", err
	}

	return chatInputTmpl.Render(map[string]any{
		"BottomsheetId": bottomsheetId,
		"Bottomsheet":   bottomsheet,
	})
}
