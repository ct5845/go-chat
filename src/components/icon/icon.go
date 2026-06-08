package icon

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ArrowDropDown   = "arrow_drop_down"
	ArrowUpward     = "arrow_upward"
	ArrowDownward   = "arrow_downward"
	WarmUp          = "arrow_warm_up"
	Cooldown        = "arrow_cool_down"
	Home            = "home"
	Close           = "close"
	Chat            = "chat_bubble"
	ContentCopy     = "content_copy"
	ChevronRight    = "chevron_right"
	CurrencyDollar  = "attach_money"
	Check           = "check"
	DataUsage       = "data_usage"
	Menu            = "menu"
	LineStartCircle = "line_start_circle"
	LineEndSquare   = "line_end_square"
	Info            = "info"
	Mic             = "mic"
	Search          = "search"
	Send            = "send"
	Stop            = "stop"
	Storage         = "storage"
	KeyboardLeft    = "keyboard_arrow_left"
	KeyboardRight   = "keyboard_arrow_right"
	KeyboardDown    = "keyboard_arrow_down"
	Schedule        = "schedule"
)

var IconFontHref string

func init() {
	var all = []string{
		ArrowUpward,
		ArrowDownward,
		ArrowDropDown,
		WarmUp,
		Cooldown,
		Home,
		Close,
		ContentCopy,
		ChevronRight,
		DataUsage,
		CurrencyDollar,
		Chat,
		Check,
		Storage,
		Info,
		LineStartCircle,
		LineEndSquare,
		Menu,
		Mic,
		Send,
		Stop,
		Search,
		KeyboardLeft,
		KeyboardRight,
		KeyboardDown,
		Schedule,
	}
	sort.Strings(all)

	IconFontHref = fmt.Sprintf("https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200&icon_names=%s&display=block", strings.Join(all, ","))
}
