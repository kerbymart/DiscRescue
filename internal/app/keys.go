package app

type KeyMap struct {
	Up         []string
	Down       []string
	Select     []string
	Back       []string
	Quit       []string
	Details    []string
	Advanced   []string
	Pause      []string
	Force      []string
	Refresh    []string
	Eject      []string
	ForceEject []string
}

func DefaultKeys() KeyMap {
	k := NewKeyMapV2()
	return KeyMap{
		Up: k.Up.Keys(), Down: k.Down.Keys(), Select: k.Select.Keys(), Back: k.Back.Keys(), Quit: k.Quit.Keys(),
		Details: k.Details.Keys(), Advanced: k.Advanced.Keys(), Pause: k.Pause.Keys(), Force: k.Force.Keys(),
		Refresh: k.Refresh.Keys(), Eject: k.Eject.Keys(), ForceEject: k.ForceEject.Keys(),
	}
}
