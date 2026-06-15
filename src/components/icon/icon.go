package icon

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

var IconFontHref string

//go:embed *.svg
var svgFiles embed.FS

var SVG = map[string]template.HTML{}

func init() {
	entries, err := fs.ReadDir(svgFiles, ".")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		content, err := svgFiles.ReadFile(entry.Name())
		if err != nil {
			panic(err)
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		SVG[name] = template.HTML(content)
	}
}

func init() {
	// Every icon name used anywhere in the app's templates must be listed
	// here so it is included in the subsetted icon font.
	names := []string{
		"add",
		"arrow_cool_down",
		"arrow_downward",
		"arrow_drop_down",
		"arrow_upward",
		"arrow_warm_up",
		"attach_money",
		"chat_bubble",
		"check",
		"chevron_right",
		"code",
		"code_off",
		"close",
		"content_copy",
		"data_usage",
		"delete",
		"forum",
		"history",
		"home",
		"info",
		"ink_pen",
		"keyboard_arrow_down",
		"keyboard_arrow_left",
		"keyboard_arrow_right",
		"line_end_square",
		"line_start_circle",
		"menu",
		"more_vert",
		"mic",
		"person",
		"schedule",
		"search",
		"send",
		"stop",
		"storage",
		"toggle_off",
		"toggle_on",
		"tools_power_drill",
	}
	// The fonts API requires icon_names to be alphabetically sorted.
	slices.Sort(names)

	IconFontHref = fmt.Sprintf("https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200&icon_names=%s&display=block", strings.Join(names, ","))
}
