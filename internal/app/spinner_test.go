package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLoadingTransitionRestartsSpinnerAfterACompletedPage(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseDrive
	model.Devices = []DeviceSummary{{Path: "/dev/sr0", DisplayName: "Optical drive"}}

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageInspectingMedia || !updated.RestartLoadingSpinner {
		t.Fatalf("expected inspecting-media spinner restart, page=%v restart=%v", updated.Page, updated.RestartLoadingSpinner)
	}
	if cmd == nil {
		t.Fatal("expected media inspection effect")
	}
}

func TestDiscoveryTransitionRestartsSpinner(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseDrive

	next, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	updated := next.(Model)
	if updated.Page != PageDiscover || !updated.RestartLoadingSpinner {
		t.Fatalf("expected discovery spinner restart, page=%v restart=%v", updated.Page, updated.RestartLoadingSpinner)
	}
	if cmd == nil {
		t.Fatal("expected discovery effect")
	}
}
