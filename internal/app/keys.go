package app

type KeyMap struct {
	Up       []string
	Down     []string
	Select   []string
	Back     []string
	Quit     []string
	Details  []string
	Advanced []string
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
	}
}
