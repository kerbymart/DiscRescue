package app

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// KeyMapV2 is the authoritative interactive vocabulary for the TUI. The
// legacy string map is derived from these bindings for transition code.
type KeyMapV2 struct {
	Up, Down, PageUp, PageDown, Select, Tab, Back, Quit, Details, Advanced, Pause, Force key.Binding
	Refresh, Eject, ForceEject                                                           key.Binding
}

func NewKeyMapV2() KeyMapV2 {
	bind := func(keys []string, helpText string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], helpText))
	}
	return KeyMapV2{
		Up: bind([]string{"up", "k"}, "move up"), Down: bind([]string{"down", "j"}, "move down"), PageUp: bind([]string{"pgup"}, "page up"), PageDown: bind([]string{"pgdown"}, "page down"),
		Select: bind([]string{"enter"}, "select"), Tab: bind([]string{"tab"}, "next field"), Back: bind([]string{"esc"}, "back"), Quit: bind([]string{"q"}, "quit"),
		Details: bind([]string{"d"}, "details"), Advanced: bind([]string{"a"}, "advanced"), Pause: bind([]string{"space"}, "pause"), Force: bind([]string{"x", "ctrl+c"}, "stop request"),
		Refresh: bind([]string{"r"}, "refresh drives"), Eject: bind([]string{"e"}, "eject"), ForceEject: bind([]string{"f"}, "force eject"),
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
		retry := k.Select
		retry.SetHelp("enter", "retry discovery")
		return pageHelpMap{groups: [][]key.Binding{{retry, k.Quit}}}
	case PageNoDrives, PageDiscoveryError, PageInspectingMedia:
		return pageHelpMap{groups: [][]key.Binding{{k.Select, k.Back, k.Quit}}}
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
	case PageChooseDrive:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Refresh, k.Eject, k.ForceEject, k.Back, k.Quit}}}
	case PageChooseAction:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Advanced, k.Back, k.Quit}}}
	case PagePriorProcessing, PageReview, PageResumeJobs, PageHistory, PageSummary:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Back, k.Quit}}}
	case PageChooseOutput:
		edit := k.Select
		edit.SetHelp("enter", "edit / accept")
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, edit, k.Tab, k.Back, k.Quit}}}
	case PagePaused:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Details, k.Back, k.Quit}}}
	case PageEjectConfirm:
		return pageHelpMap{groups: [][]key.Binding{{k.Up, k.Down, k.Select, k.Back}}}
	default:
		return pageHelpMap{groups: [][]key.Binding{common}}
	}
}

func pageHelpForModel(m Model) pageHelpMap {
	if m.Page == PagePausing && m.Recovery.ForceStopAvailable {
		k := NewKeyMapV2()
		force := k.Force
		force.SetHelp("x", "force stop")
		return pageHelpMap{groups: [][]key.Binding{{k.Details, force}}}
	}
	result := pageHelp(m.Page)
	if m.noticeHasTechnicalDetail() && m.Page != PageDetails && m.Page != PageRecovering && m.Page != PagePausing && m.Page != PagePaused && m.Page != PageSummary {
		k := NewKeyMapV2()
		if len(result.groups) == 0 {
			result.groups = [][]key.Binding{{k.Details}}
		} else {
			result.groups[0] = append(result.groups[0], k.Details)
		}
	}
	return result
}
