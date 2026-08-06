package app

import (
	"fmt"
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
		RequestID: model.ActiveDiscoveryRequest,
		Devices:   []DeviceSummary{{Path: "D:/dev/cdrom", DisplayName: "DVD Drive", Status: "ready"}},
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

func TestMediaIdentifiedMovesToChooseAction(t *testing.T) {
	model := NewModel()
	model.ActiveMediaRequest = 1

	next, cmd := model.Update(MediaIdentifiedMsg{
		RequestID: 1,
		Identity: ContentIdentityViewModel{
			Summary: "Optical media detected.",
			Detail:  "DVD-ROM, 4.38 GiB",
		},
		LogicalSectorSize:   2048,
		CapacitySectors:     1024,
		Recoverable:         true,
		SuggestedOutputPath: "disc.iso",
	})
	updated := next.(Model)
	if cmd != nil {
		t.Fatalf("expected no follow-up command, got %#v", cmd)
	}

	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseAction)
	}
	if !updated.MediaRecoverable {
		t.Fatal("expected recoverable media")
	}
}

func TestPriorProcessingLookupUnavailableKeepsActionPageReachable(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction

	next, _ := model.Update(PriorProcessingLookupMsg{Err: fmt.Errorf("history lookup is unavailable in this build")})
	updated := next.(Model)

	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseAction)
	}
	if updated.Notice == nil || updated.Notice.Text != "History lookup is unavailable in this build." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
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

	if updated.Page != PageReview {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageReview)
	}
	if requested.Kind != EffectStartJob {
		t.Fatalf("unexpected effect kind: %q", requested.Kind)
	}
}

func TestJobStartFailedKeepsReviewActionable(t *testing.T) {
	model := NewModel()
	model.Page = PageReview

	next, _ := model.Update(JobStartFailedMsg{Err: fmt.Errorf("starting recovery is not connected to real device and image work yet")})
	updated := next.(Model)

	if updated.Page != PageReview {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageReview)
	}
	if updated.Notice == nil || updated.Notice.Severity != SeverityWarning {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestChooseActionRequiresRecoverableMedia(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.MediaRecoverable = false
	model.MediaRecoverabilityNote = "No mounted data media is available."

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page=%v", updated.Page)
	}
	if updated.Notice == nil || updated.Notice.Text != "No mounted data media is available." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestChooseActionStartMovesToReview(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.MediaRecoverable = true

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageReview {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageReview)
	}
}

func TestReviewChooseAnotherDriveReturnsToDriveList(t *testing.T) {
	model := NewModel()
	model.Page = PageReview
	model.Cursor = 1

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseDrive {
		t.Fatalf("unexpected transition: %+v", updated)
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

func TestSpaceDuringRecoveryShowsTruthfulPauseNotice(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	updated := next.(Model)
	if cmd != nil {
		t.Fatalf("expected no pause command, got %#v", cmd)
	}
	if updated.Page != PageRecovering {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageRecovering)
	}
	if updated.Notice == nil || updated.Notice.Text != "Pause is not implemented for the current recovery backend." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestQuitFromRecoveryOpensStopConfirmation(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, _ := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	updated := next.(Model)
	if updated.Page != PageStopConfirm || updated.PreviousPage != PageRecovering {
		t.Fatalf("unexpected stop-confirm transition: %+v", updated)
	}
}

func TestStopConfirmationDefaultRequestsCheckpointedStop(t *testing.T) {
	model := NewModel()
	model.Page = PageStopConfirm
	model.PreviousPage = PageRecovering

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	requested := cmd().(EffectRequestedMsg)
	if updated.Page != PageStopConfirm {
		t.Fatalf("expected stop-confirm page to remain active until result, got %v", updated.Page)
	}
	if requested.Kind != EffectStopJob {
		t.Fatalf("unexpected stop request: %+v", requested)
	}
}

func TestProgressAndWorkerStatusRemainResponsive(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, _ := model.Update(ProgressMsg{Snapshot: ProgressSnapshot{
		Phase:            "Finding readable areas",
		RecoveredSectors: 120,
		TotalSectors:     240,
		Status:           "Reading difficult areas.",
		Remaining:        "1.42 GiB of 4.38 GiB remaining",
		ETA:              "about 7 minutes",
		LastIssue:        []string{"Last issue: sector 1,891,840 could not be read.", "It will be tried again during the recovery pass."},
		OutputPath:       "D:/Archives/archive-disc.iso",
	}})
	updated := next.(Model)
	if updated.Recovery.Phase != "Finding readable areas" {
		t.Fatalf("unexpected recovery phase: %q", updated.Recovery.Phase)
	}

	next, _ = updated.Update(WorkerUnresponsiveMsg{Since: 3 * time.Second})
	updated = next.(Model)
	if updated.Notice == nil || updated.Notice.Severity != SeverityWarning {
		t.Fatalf("expected warning notice, got %+v", updated.Notice)
	}

	next, _ = updated.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	afterQuit := next.(Model)
	if afterQuit.Page != PageStopConfirm {
		t.Fatalf("expected stop confirmation after worker warning, got %v", afterQuit.Page)
	}
}

func TestJobStoppedMovesToSummaryWithPrimaryChoiceFocused(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, _ := model.Update(JobStoppedMsg{Summary: JobSummary{
		Outcome:          "Recovery complete",
		ImagePath:        "D:/Archives/archive-disc.iso",
		MapPath:          "D:/Archives/archive-disc.drmap",
		NextAction:       "Choose another drive to start another recovery",
		RecoveredSectors: 2295104,
		TotalSectors:     2295104,
		Duration:         "31 minutes",
	}})
	updated := next.(Model)
	if updated.Page != PageSummary || updated.Cursor != 0 {
		t.Fatalf("unexpected summary state: %+v", updated)
	}
	if updated.Recovery.Status != "Recovery complete" {
		t.Fatalf("unexpected summary outcome: %q", updated.Recovery.Status)
	}
	if updated.Summary.Duration != "31 minutes" {
		t.Fatalf("unexpected summary payload: %+v", updated.Summary)
	}
}
