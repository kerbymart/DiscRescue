package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUpdateWindowSize(t *testing.T) {
	model := NewModel()

	next, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	updated := next.(Model)

	if updated.Width != 80 || updated.Height != 24 {
		t.Fatalf("unexpected size: got %dx%d", updated.Width, updated.Height)
	}
}

func TestUpdateQuitKey(t *testing.T) {
	model := NewModel()

	next, cmd := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	updated := next.(Model)

	if !updated.ShouldQuit {
		t.Fatal("expected quit flag to be set")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}
