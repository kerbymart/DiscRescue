package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDriveListOwnsKeyboardNavigation(t *testing.T) {
	model := NewModel()
	next, _ := model.Update(DevicesDiscoveredMsg{
		RequestID: model.ActiveDiscoveryRequest,
		Devices: []DeviceSummary{
			{ID: "a", Path: "/dev/sr0", DisplayName: "Drive A"},
			{ID: "b", Path: "/dev/sr1", DisplayName: "Drive B"},
		},
	})
	model = next.(Model)
	model.Page = PageChooseDrive
	model.DriveList.Select(0)

	next, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	updated := next.(Model)
	if updated.DriveList.Index() != 1 || updated.Cursor != 1 {
		t.Fatalf("drive list did not own navigation: index=%d cursor=%d", updated.DriveList.Index(), updated.Cursor)
	}
}

func TestOutputEditingRoutesInputToExactlyOneFocusedField(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Setup.OutputEditing = true
	model.Setup.ActiveOutputField = OutputFieldDirectory
	model.DirectoryInput.Focus()
	model.FileNameInput.Blur()
	model.DirectoryInput.SetValue("/archive")
	model.FileNameInput.SetValue("disc.iso")

	next, _ := model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	updated := next.(Model)
	if !strings.Contains(updated.DirectoryInput.Value(), "x") || updated.FileNameInput.Value() != "disc.iso" {
		t.Fatalf("directory focus routing failed: directory=%q filename=%q", updated.DirectoryInput.Value(), updated.FileNameInput.Value())
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	updated = next.(Model)
	next, _ = updated.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	updated = next.(Model)
	if !strings.Contains(updated.DirectoryInput.Value(), "x") || !strings.HasSuffix(updated.FileNameInput.Value(), "y") {
		t.Fatalf("filename focus routing failed: directory=%q filename=%q", updated.DirectoryInput.Value(), updated.FileNameInput.Value())
	}
}

func TestDetailsViewportOwnsScrolling(t *testing.T) {
	model := NewModel()
	model.Page = PageDetails
	model.Width, model.Height = 60, 18
	model.Details = DetailsViewModel{Lines: strings.Split(strings.Repeat("technical detail\n", 30), "\n")}
	next, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	model = next.(Model)
	before := model.DetailsViewport.YOffset()
	next, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	updated := next.(Model)
	if updated.DetailsViewport.YOffset() <= before {
		t.Fatalf("details viewport did not scroll: before=%d after=%d", before, updated.DetailsViewport.YOffset())
	}
}
