package app

type KeyMap struct {
	Quit  string
	Start string
}

func DefaultKeys() KeyMap {
	return KeyMap{
		Quit:  "q",
		Start: "enter",
	}
}
