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

type pageHelpMap struct{ groups [][]key.Binding }

func (p pageHelpMap) ShortHelp() []key.Binding {
	var out []key.Binding
	for _, group := range p.groups {
		out = append(out, group...)
	}
	return out
}

func (p pageHelpMap) FullHelp() [][]key.Binding { return p.groups }

func pageHelp(page Page) pageHelpMap {
	k := NewKeyMapV2()
	common := []key.Binding{k.Up, k.Down, k.Select, k.Back, k.Quit}
	switch page {
	case PageDiscover:
		return pageHelpMap{groups: [][]key.Binding{{k.Quit}}}
	case PageRecovering, PagePausing:
		return pageHelpMap{groups: [][]key.Binding{{k.Pause, k.Details, k.Quit}}}
	case PageDetails:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Back, k.Quit}}}
	case PageChooseOutput:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Back, k.Quit}}}
	default:
		return pageHelpMap{groups: [][]key.Binding{common}}
	}
}
