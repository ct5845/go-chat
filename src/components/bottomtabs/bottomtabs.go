package bottomtabs

import (
	_ "embed"
	"fmt"
	"html/template"
	"ct-go-chat/src/components/component"
)

var (
	//go:embed bottomtabs.html
	bottomTabsHTML string
	//go:embed bottomtabs.js
	bottomTabsJS string
	bottomTabsTpl = component.WithAlpine("bottomtabs.html", bottomTabsHTML, bottomTabsJS)
)

type Tab struct {
	Icon    string
	Label   string
	Href    string
	Attrs   template.HTMLAttr
	Active  bool
	Primary bool
}

type Options struct {
	Tabs []Tab
}

type templateOptions struct {
	Tabs  []template.HTML
	Style template.HTMLAttr
}

func Render(options Options) (template.HTML, error) {
	return bottomTabsTpl.Render(toTemplateOptions(options))
}

func MustRender(options Options) template.HTML {
	return bottomTabsTpl.MustRender(toTemplateOptions(options))
}

func toTemplateOptions(options Options) templateOptions {
	tabs := make([]template.HTML, len(options.Tabs))
	for i, t := range options.Tabs {
		tabs[i] = t.render()
	}
	return templateOptions{
		Tabs:  tabs,
		Style: template.HTMLAttr(fmt.Sprintf("style=\"grid-template-columns: repeat(%d, 1fr)\"", len(options.Tabs))),
	}
}

func (t Tab) render() template.HTML {
	tag := "button"
	if t.Href != "" {
		tag = "a"
	}

	iconClass := "icon-wght-200 icon-opsz-20"
	if t.Active {
		iconClass = "icon-fill-1 icon-wght-800 icon-opsz-20 drop-shadow"
	}
	iconHTML := fmt.Sprintf(`<span aria-hidden="true" class="icon %s">%s</span>`, iconClass, t.Icon)

	stateClass := "border-outline-variant"
	labelClass := ` class="text-sm"`
	if t.Active {
		stateClass = "text-on-primary-container bg-primary-container"
		labelClass = ` class="font-bold text-sm"`
	}

	href := ""
	if t.Href != "" {
		href = fmt.Sprintf(` href="%s"`, t.Href)
	}

	attrs := ""
	if t.Attrs != "" {
		attrs = fmt.Sprintf(` %s`, t.Attrs)
	}

	return template.HTML(fmt.Sprintf(`
<%s%s%s class="flex flex-col items-center justify-center border-t-2 p-2 %s">
  %s
  <span%s>%s</span>
</%s>`, tag, href, attrs, stateClass, iconHTML, labelClass, t.Label, tag))
}
