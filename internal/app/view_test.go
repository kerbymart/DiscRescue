package app

import (
	"strings"
	"testing"
)

func TestViewTooSmall(t *testing.T) {
	model := NewModel()
	model.Width = 39
	model.Height = 11

	view := model.View().Content
	if !strings.Contains(view, "Resize to at least 40x12") {
		t.Fatalf("unexpected small-window view: %q", view)
	}
}

func TestViewChooseDriveShowsOneColumnList(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageChooseDrive
	model.Devices = []DeviceSummary{
		{Path: "D:/dev/cdrom0", DisplayName: "DVD Drive", Status: "ready"},
		{Path: "D:/dev/cdrom1", DisplayName: "Blu-ray Drive", Status: "busy"},
	}

	view := model.View().Content
	if !strings.Contains(view, "Choose a drive") {
		t.Fatalf("expected choose-drive title, got %q", view)
	}
	if !strings.Contains(view, "> DVD Drive") {
		t.Fatalf("expected focused device row, got %q", view)
	}
	if strings.Contains(view, "│") || strings.Contains(view, "┌") {
		t.Fatalf("expected plain one-column layout without panels, got %q", view)
	}
}

func TestViewPriorProcessingShowsMatchingContentsLanguage(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PagePriorProcessing
	model.Identity = ContentIdentityViewModel{
		Summary: "Matching contents were processed before",
		Detail:  "Archived successfully",
	}
	model.PriorRecords = []PriorProcessingRecord{{
		Title:  "History",
		Detail: "Previous files not found",
	}}

	view := model.View().Content
	if !strings.Contains(view, "Matching contents were processed before") {
		t.Fatalf("expected matching-contents wording, got %q", view)
	}
	if strings.Contains(strings.ToLower(view), "same physical disc") {
		t.Fatalf("unexpected physical-identity wording: %q", view)
	}
}

func TestViewRecoveringUsesAltScreenAndNoTelemetryTable(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		Phase:            "Adaptive recovery",
		RecoveredSectors: 120,
		TotalSectors:     240,
		Status:           "Reading difficult areas.",
	}

	view := model.View()
	if !view.AltScreen {
		t.Fatal("expected recovering page to use the alternate screen")
	}
	if strings.Contains(strings.ToLower(view.Content), "throughput") || strings.Contains(strings.ToLower(view.Content), "chart") {
		t.Fatalf("unexpected telemetry content: %q", view.Content)
	}
}

func TestViewDetailsUsesAltScreen(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageDetails
	model.Details = DetailsViewModel{Lines: []string{"Drive: D:/dev/cdrom0", "Worker: responsive"}}

	view := model.View()
	if !view.AltScreen {
		t.Fatal("expected details page to use the alternate screen")
	}
	if !strings.Contains(view.Content, "Drive: D:/dev/cdrom0") {
		t.Fatalf("unexpected details content: %q", view.Content)
	}
}
