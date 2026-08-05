package app

type Screen string

const (
	ScreenWelcome Screen = "welcome"
)

type Model struct {
	Screen       Screen
	Width        int
	Height       int
	Notice       string
	StatusLine   string
	ShouldQuit   bool
	CurrentFocus string
}

func NewModel() Model {
	return Model{
		Screen:       ScreenWelcome,
		Notice:       "Simulator-first bootstrap is ready.",
		StatusLine:   "Press q to quit.",
		CurrentFocus: "start",
	}
}
