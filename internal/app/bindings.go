package app

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// KeyMapV2 is the authoritative interactive vocabulary for the TUI. The
// legacy string map remains available to keep the transition incremental.
type KeyMapV2 struct {
	Up, Down, PageUp, PageDown, Select, Tab, Back, Quit, Details, Advanced, Pause, Force key.Binding
}

func NewKeyMapV2() KeyMapV2 {
	bind := func(keys []string, helpText string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], helpText))
	}
	return KeyMapV2{
		Up: bind([]string{"up", "k"}, "move up"), Down: bind([]string{"down", "j"}, "move down"), PageUp: bind([]string{"pgup"}, "page up"), PageDown: bind([]string{"pgdown"}, "page down"),
		Select: bind([]string{"enter"}, "select"), Tab: bind([]string{"tab"}, "next field"), Back: bind([]string{"esc"}, "back"), Quit: bind([]string{"q"}, "quit"),
		Details: bind([]string{"d"}, "details"), Advanced: bind([]string{"a"}, "advanced"), Pause: bind([]string{"space"}, "pause"), Force: bind([]string{"ctrl+c"}, "stop now"),
	}
}

// FooterHelp creates the reusable footer renderer for a page's active keys.
func FooterHelp(full bool) help.Model {
	h := help.New()
	h.ShowAll = full
	h.ShortSeparator = "  •  "
	h.Styles.ShortKey = h.Styles.ShortKey.Foreground(lipgloss.Color("#A78BFA")).Bold(true)
	h.Styles.ShortDesc = h.Styles.ShortDesc.Foreground(lipgloss.Color("#9690A8"))
	h.Styles.ShortSeparator = h.Styles.ShortSeparator.Foreground(lipgloss.Color("#514766"))
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
	case PageRecovering:
		stop := k.Quit
		stop.SetHelp("q", "stop")
		return pageHelpMap{groups: [][]key.Binding{{k.Pause, k.Details, stop}}}
	case PagePausing:
		stop := k.Quit
		stop.SetHelp("q", "stop")
		return pageHelpMap{groups: [][]key.Binding{{k.Details, stop}}}
	case PageDetails:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.PageUp, k.PageDown, k.Back}}}
	case PageChooseOutput:
		edit := k.Select
		edit.SetHelp("enter", "edit / accept")
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, edit, k.Tab, k.Back}}}
	case PageChooseDrive:
		refresh := key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh drives"))
		eject := key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "eject"))
		force := key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "force eject"))
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, refresh, eject, force, k.Back, k.Quit}}}
	case PageEjectConfirm:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Back}}}
	default:
		return pageHelpMap{groups: [][]key.Binding{common}}
	}
}
