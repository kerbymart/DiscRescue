package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestInitRequestsDeviceDiscovery(t *testing.T) {
	model := NewModel()

	msg := model.Init()()
	requested, ok := msg.(EffectRequestedMsg)
	if !ok {
		t.Fatalf("expected effect request, got %T", msg)
	}
	if requested.Kind != EffectDiscoverDevices {
		t.Fatalf("unexpected effect kind: %q", requested.Kind)
	}
}

func TestUpdateWindowSize(t *testing.T) {
	model := NewModel()

	next, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	updated := next.(Model)

	if updated.Width != 80 || updated.Height != 24 {
		t.Fatalf("unexpected size: got %dx%d", updated.Width, updated.Height)
	}
}

func TestDevicesDiscoveredMovesToChooseDrive(t *testing.T) {
	model := NewModel()

	next, _ := model.Update(DevicesDiscoveredMsg{
		Devices: []DeviceSummary{{Path: "D:/dev/cdrom", DisplayName: "DVD Drive", Status: "ready"}},
	})
	updated := next.(Model)

	if updated.Page != PageChooseDrive {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseDrive)
	}
	if len(updated.Devices) != 1 {
		t.Fatalf("unexpected devices: %+v", updated.Devices)
	}
}

func TestSelectDriveRequestsIdentifyEffect(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseDrive
	model.Devices = []DeviceSummary{{Path: "D:/dev/cdrom", DisplayName: "DVD Drive", Status: "ready"}}

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	msg := cmd()
	requested := msg.(EffectRequestedMsg)

	if updated.Identity.Summary == "" {
		t.Fatal("expected identify summary to be set")
	}
	if requested.Kind != EffectIdentifyMedia || requested.DevicePath != "D:/dev/cdrom" {
		t.Fatalf("unexpected identify request: %+v", requested)
	}
}

func TestMediaIdentifiedMovesToPriorProcessingAndRequestsLookup(t *testing.T) {
	model := NewModel()

	next, cmd := model.Update(MediaIdentifiedMsg{
		Identity: ContentIdentityViewModel{
			Summary: "Matching contents were processed before",
			Detail:  "DVD-ROM, 4.38 GiB",
		},
	})
	updated := next.(Model)
	requested := cmd().(EffectRequestedMsg)

	if updated.Page != PagePriorProcessing {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PagePriorProcessing)
	}
	if requested.Kind != EffectLookupHistory {
		t.Fatalf("unexpected effect kind: %q", requested.Kind)
	}
}

func TestEnterAdvancesSetupFlow(t *testing.T) {
	model := NewModel()
	model.Page = PagePriorProcessing
	model.PriorView = PriorProcessingViewModel{
		Kind:    PriorProcessingStrongCompleted,
		Title:   "Matching disc contents were processed before",
		Body:    []string{"Archived successfully on 2 August 2026"},
		Options: []string{"Verify the previous archive", "Start another capture", "View previous job", "Edit physical-copy label", "Back"},
	}

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page after prior processing: got %v want %v", updated.Page, PageChooseAction)
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = next.(Model)
	if updated.Page != PageReview {
		t.Fatalf("unexpected page after action: got %v want %v", updated.Page, PageReview)
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = next.(Model)
	if updated.Page != PageRecovering {
		t.Fatalf("unexpected page after review: got %v want %v", updated.Page, PageRecovering)
	}
}

func TestPriorProcessingSafeDefaultOptions(t *testing.T) {
	model := NewModel()
	model.Page = PagePriorProcessing
	model.PriorView = PriorProcessingViewModel{
		Kind:    PriorProcessingStrongCompleted,
		Title:   "Matching disc contents were processed before",
		Body:    []string{"Archived successfully on 2 August 2026"},
		Options: []string{"Verify the previous archive", "Start another capture", "View previous job", "Edit physical-copy label", "Back"},
	}

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page after prior-processing default: got %v want %v", updated.Page, PageChooseAction)
	}
}

func TestReviewEnterRequestsStartJob(t *testing.T) {
	model := NewModel()
	model.Page = PageReview

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	requested := cmd().(EffectRequestedMsg)

	if updated.Page != PageRecovering {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageRecovering)
	}
	if requested.Kind != EffectStartJob {
		t.Fatalf("unexpected effect kind: %q", requested.Kind)
	}
}

func TestChooseActionSupportsVerifyAndMergeSetup(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction

	model.Cursor = 2
	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseOutput || updated.Setup.ActionLabel != "Verify an existing image" {
		t.Fatalf("unexpected verify flow: page=%v setup=%+v", updated.Page, updated.Setup)
	}

	model = NewModel()
	model.Page = PageChooseAction
	model.Cursor = 3
	next, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = next.(Model)
	if updated.Page != PageChooseOutput || updated.Setup.ActionLabel != "Merge recovery captures" {
		t.Fatalf("unexpected merge flow: page=%v setup=%+v", updated.Page, updated.Setup)
	}
}

func TestReviewChangeMethodAndLabelRemainInline(t *testing.T) {
	model := NewModel()
	model.Page = PageReview

	model.Cursor = 2
	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Setup.MethodLabel != "Fast recovery" {
		t.Fatalf("unexpected method label: %q", updated.Setup.MethodLabel)
	}
	if updated.Page != PageReview {
		t.Fatalf("expected review page to remain active, got %v", updated.Page)
	}

	updated.Cursor = 3
	next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = next.(Model)
	if updated.Setup.CopyLabel != "Shelf B · Disc 14" {
		t.Fatalf("unexpected copy label: %q", updated.Setup.CopyLabel)
	}
}

func TestReviewAdvancedSettingsStaysSeparateFromMainFlow(t *testing.T) {
	model := NewModel()
	model.Page = PageReview
	model.Cursor = 4

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageAdvanced || updated.PreviousPage != PageReview {
		t.Fatalf("unexpected advanced transition: %+v", updated)
	}
}

func TestDetailsKeyOpensAndEscReturns(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, _ := model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	updated := next.(Model)
	if updated.Page != PageDetails || updated.PreviousPage != PageRecovering {
		t.Fatalf("unexpected details transition: %+v", updated)
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated = next.(Model)
	if updated.Page != PageRecovering {
		t.Fatalf("unexpected page after escape: got %v want %v", updated.Page, PageRecovering)
	}
}

func TestProgressAndWorkerStatusRemainResponsive(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, _ := model.Update(ProgressMsg{Snapshot: ProgressSnapshot{
		Phase:            "Adaptive recovery",
		RecoveredSectors: 120,
		TotalSectors:     240,
		Status:           "Reading difficult areas.",
	}})
	updated := next.(Model)
	if updated.Recovery.Phase != "Adaptive recovery" {
		t.Fatalf("unexpected recovery phase: %q", updated.Recovery.Phase)
	}

	next, _ = updated.Update(WorkerUnresponsiveMsg{Since: 3 * time.Second})
	updated = next.(Model)
	if updated.Notice == nil || updated.Notice.Severity != SeverityWarning {
		t.Fatalf("expected warning notice, got %+v", updated.Notice)
	}

	next, cmd := updated.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	afterQuit := next.(Model)
	if !afterQuit.Quitting || cmd == nil {
		t.Fatal("expected ui to remain responsive after worker warning")
	}
}
