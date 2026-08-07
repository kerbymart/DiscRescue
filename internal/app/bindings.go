package app

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// KeyMapV2 is the authoritative interactive vocabulary for the TUI. The
// legacy string map remains available to keep the transition incremental.
type KeyMapV2 struct {
	Up, Down, Select, Back, Quit, Details, Advanced, Pause, Force key.Binding
}

func NewKeyMapV2() KeyMapV2 {
	bind := func(keys []string, helpText string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], helpText))
	}
	return KeyMapV2{
		Up: bind([]string{"up", "k"}, "move up"), Down: bind([]string{"down", "j"}, "move down"),
		Select: bind([]string{"enter"}, "select"), Back: bind([]string{"esc"}, "back"), Quit: bind([]string{"q"}, "quit"),
		Details: bind([]string{"d"}, "details"), Advanced: bind([]string{"a"}, "advanced"), Pause: bind([]string{"space"}, "pause"), Force: bind([]string{"ctrl+c"}, "stop now"),
	}
}

// FooterHelp creates the reusable footer renderer for a page's active keys.
func FooterHelp(full bool) help.Model {
	h := help.New()
	h.ShowAll = full
	return h
}
