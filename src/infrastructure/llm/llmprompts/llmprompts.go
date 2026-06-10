package llmprompts

import (
	"bytes"
	_ "embed"
	"text/template"
)

var (
	//go:embed identity.txt
	identityTxt string
)

func GetSystemPrompt() (string, error) {
	systemPromptTpl, err := template.New("SystemPrompt").Parse(identityTxt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := systemPromptTpl.Execute(&buf, nil); err != nil {
		return "", err
	}

	return buf.String(), nil
}
