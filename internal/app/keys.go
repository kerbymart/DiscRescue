package app

type KeyMap struct {
	Up       []string
	Down     []string
	Select   []string
	Back     []string
	Quit     []string
	Details  []string
	Advanced []string
	Pause    []string
	Force    []string
	Refresh  []string
}

func DefaultKeys() KeyMap {
	return KeyMap{
		Up:       []string{"up", "k"},
		Down:     []string{"down", "j"},
		Select:   []string{"enter"},
		Back:     []string{"esc"},
		Quit:     []string{"q"},
		Details:  []string{"d"},
		Advanced: []string{"a"},
		Pause:    []string{"space"},
		Force:    []string{"ctrl+c"},
		Refresh:  []string{"r"},
	}
}
