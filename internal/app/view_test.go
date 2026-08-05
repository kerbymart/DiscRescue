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
	model.PriorView = PriorProcessingViewModel{
		Kind:      PriorProcessingStrongCompleted,
		Title:     "Matching disc contents were processed before",
		Body:      []string{"Archived successfully on 2 August 2026"},
		ImagePath: "D:/Archives/Family-Video-03.iso",
		CopyLabel: "Shelf B · Disc 14",
		Options:   []string{"Verify the previous archive", "Start another capture", "View previous job", "Edit physical-copy label", "Back"},
	}

	view := model.View().Content
	if !strings.Contains(view, "Matching disc contents were processed before") {
		t.Fatalf("expected matching-contents wording, got %q", view)
	}
	if strings.Contains(strings.ToLower(view), "same physical disc") {
		t.Fatalf("unexpected physical-identity wording: %q", view)
	}
	if !strings.Contains(view, "> Verify the previous archive") {
		t.Fatalf("expected safe default action, got %q", view)
	}
}

func TestViewActionPageShowsHistoryLineAndDiscSummary(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageChooseAction
	model.Identity = ContentIdentityViewModel{Detail: "DVD-ROM, 4.38 GiB"}
	model.PriorView = PriorProcessingViewModel{
		Kind:        PriorProcessingNone,
		HistoryLine: "History: no matching contents found on this computer",
	}

	view := model.View().Content
	if !strings.Contains(view, "History: no matching contents found on this computer") {
		t.Fatalf("expected history line, got %q", view)
	}
	if !strings.Contains(view, "Disc: DVD-ROM, 4.38 GiB") {
		t.Fatalf("expected disc summary, got %q", view)
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
