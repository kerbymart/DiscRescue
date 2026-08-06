package app

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
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
	if cmd == nil {
		t.Fatal("expected history lookup follow-up command")
	}
	requested := cmd().(EffectRequestedMsg)
	if requested.Kind != EffectLookupHistory {
		t.Fatalf("unexpected follow-up effect: %+v", requested)
	}

	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseAction)
	}
	if !updated.MediaRecoverable {
		t.Fatal("expected recoverable media")
	}
	if updated.Setup.OutputDirectory != "." || updated.Setup.OutputFileName != "disc.iso" {
		t.Fatalf("unexpected output parts: %+v", updated.Setup)
	}
	if cmd == nil {
		t.Fatal("expected history lookup effect")
	}
}

func TestPriorProcessingLookupUnavailableKeepsActionPageReachable(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.ActiveLookupRequest = 3

	next, _ := model.Update(PriorProcessingLookupMsg{RequestID: 3, Err: fmt.Errorf("history lookup failed")})
	updated := next.(Model)

	if updated.Page != PageChooseAction {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseAction)
	}
	if updated.Notice == nil || updated.Notice.Text != "history lookup failed" {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestPriorProcessingLookupUpdatesHistoryLine(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.ActiveLookupRequest = 4

	next, _ := model.Update(PriorProcessingLookupMsg{
		RequestID: 4,
		View: PriorProcessingViewModel{
			Kind:        PriorProcessingStrongResumable,
			HistoryLine: "Found 1 resumable matching recoveries in D:/Archives.",
		},
	})
	updated := next.(Model)
	if updated.PriorView.HistoryLine != "Found 1 resumable matching recoveries in D:/Archives." {
		t.Fatalf("unexpected prior view: %+v", updated.PriorView)
	}
}

func TestPriorProcessingLookupWithResumableMatchMovesToInterstitial(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.ActiveLookupRequest = 5

	next, _ := model.Update(PriorProcessingLookupMsg{
		RequestID: 5,
		View: PriorProcessingViewModel{
			Kind:        PriorProcessingStrongResumable,
			Title:       "Matching contents were found on this computer",
			HistoryLine: "Found 1 resumable matching recoveries in D:/Archives.",
			Options:     []string{"Resume the matching recovery", "Start a new recovery instead", "Choose another drive"},
		},
		Jobs: []ResumableJobViewModel{{
			OutputPath: "D:/Archives/disc.iso",
			MapPath:    "D:/Archives/disc.drmap",
			Detail:     "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
		}},
	})
	updated := next.(Model)
	if updated.Page != PagePriorProcessing {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PagePriorProcessing)
	}
	if len(updated.ResumeJobs) != 1 {
		t.Fatalf("unexpected resume jobs: %+v", updated.ResumeJobs)
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

func TestPriorProcessingResumeDefaultMovesToReview(t *testing.T) {
	model := NewModel()
	model.Page = PagePriorProcessing
	model.PriorView = PriorProcessingViewModel{
		Kind:    PriorProcessingStrongResumable,
		Title:   "Matching contents were found on this computer",
		Body:    []string{"Use Resume an unfinished recovery to continue from the saved map."},
		Options: []string{"Resume the matching recovery", "Start a new recovery instead", "Choose another drive"},
	}
	model.ResumeJobs = []ResumableJobViewModel{{
		OutputPath: "D:/Archives/disc.iso",
		MapPath:    "D:/Archives/disc.drmap",
		Detail:     "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
	}}

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageReview {
		t.Fatalf("unexpected page after prior processing resume: got %v want %v", updated.Page, PageReview)
	}
	if updated.Setup.ActionLabel != "Resume recovery" || !updated.Setup.ResumeReady {
		t.Fatalf("unexpected setup state: %+v", updated.Setup)
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

func TestJobStartedPreservesResumedProgress(t *testing.T) {
	model := NewModel()

	next, _ := model.Update(JobStartedMsg{
		JobID:             "job-1",
		OutputPath:        "D:/Archives/archive-disc.iso",
		Phase:             "Resuming optical recovery",
		Status:            "Resuming from the saved recovery map.",
		TotalSectors:      240,
		RecoveredSectors:  120,
		UnreadableSectors: 3,
	})
	updated := next.(Model)
	if updated.Page != PageRecovering {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageRecovering)
	}
	if updated.Recovery.RecoveredSectors != 120 || updated.Recovery.UnreadableSectors != 3 {
		t.Fatalf("unexpected recovery state: %+v", updated.Recovery)
	}
}

func TestJobStartFailedKeepsReviewActionable(t *testing.T) {
	model := NewModel()
	model.Page = PageReview

	next, _ := model.Update(JobStartFailedMsg{Err: fmt.Errorf("starting recovery is not connected to real device and image work yet")})
	updated := next.(Model)

	if updated.Page != PageChooseOutput {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseOutput)
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

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseOutput {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseOutput)
	}
	if updated.Notice == nil || updated.Notice.Text != "Checking the suggested output path." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
	requested := cmd().(EffectRequestedMsg)
	if requested.Kind != EffectInspectTarget {
		t.Fatalf("unexpected effect request: %+v", requested)
	}
}

func TestChooseActionResumeRequestsJobDiscovery(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.MediaRecoverable = true
	model.Cursor = 1
	model.Setup.OutputDirectory = "D:/Archives"

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageResumeJobs {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageResumeJobs)
	}
	requested := cmd().(EffectRequestedMsg)
	if requested.Kind != EffectFindResumeJobs || requested.BasePath != "D:/Archives" {
		t.Fatalf("unexpected effect request: %+v", requested)
	}
}

func TestChooseActionBrowseHistoryRequestsFolderScan(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseAction
	model.MediaRecoverable = true
	model.Cursor = 2
	model.Setup.OutputDirectory = "D:/Archives"

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageHistory {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageHistory)
	}
	requested := cmd().(EffectRequestedMsg)
	if requested.Kind != EffectBrowseHistory || requested.BasePath != "D:/Archives" {
		t.Fatalf("unexpected effect request: %+v", requested)
	}
}

func TestReviewChooseAnotherDriveReturnsToDriveList(t *testing.T) {
	model := NewModel()
	model.Page = PageReview
	model.Cursor = 2

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseDrive {
		t.Fatalf("unexpected transition: %+v", updated)
	}
}

func TestChooseOutputEnterRequestsTargetInspection(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Cursor = 2
	model.Setup.OutputDirectory = "D:/Archives"
	model.Setup.OutputFileName = "disc.iso"
	syncOutputPath(&model.Setup)

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageChooseOutput {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseOutput)
	}
	requested := cmd().(EffectRequestedMsg)
	if requested.Kind != EffectInspectTarget || requested.OutputPath != filepath.Join("D:/Archives", "disc.iso") {
		t.Fatalf("unexpected effect request: %+v", requested)
	}
	if updated.Notice == nil || updated.Notice.Text != "Checking the selected output path." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestChooseOutputTabSwitchesActiveField(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Setup.ActiveOutputField = OutputFieldDirectory
	model.Setup.OutputEditing = true

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	updated := next.(Model)
	if updated.Setup.ActiveOutputField != OutputFieldFileName {
		t.Fatalf("unexpected active field: %v", updated.Setup.ActiveOutputField)
	}
}

func TestChooseOutputTypingEditsActiveFieldAndBuildsFullPath(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Setup.OutputDirectory = "D:/Archives"
	model.Setup.OutputFileName = "disc"
	model.Setup.ActiveOutputField = OutputFieldFileName
	model.Setup.OutputEditing = true
	syncOutputPath(&model.Setup)

	next, _ := model.Update(tea.KeyPressMsg{Text: ".iso"})
	updated := next.(Model)
	if updated.Setup.OutputFileName != "disc.iso" {
		t.Fatalf("unexpected file name: %q", updated.Setup.OutputFileName)
	}
	if updated.Setup.OutputPath != filepath.Join("D:/Archives", "disc.iso") {
		t.Fatalf("unexpected output path: %q", updated.Setup.OutputPath)
	}
}

func TestChooseOutputEnterOnFolderStartsEditing(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Cursor = 0

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if cmd != nil {
		t.Fatalf("expected no command, got %#v", cmd)
	}
	if !updated.Setup.OutputEditing || updated.Setup.ActiveOutputField != OutputFieldDirectory {
		t.Fatalf("unexpected editing state: %+v", updated.Setup)
	}
}

func TestChooseOutputEnterWhileEditingStopsEditing(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Cursor = 1
	model.Setup.OutputDirectory = "D:/Archives"
	model.Setup.OutputFileName = "disc.iso"
	model.Setup.ActiveOutputField = OutputFieldFileName
	model.Setup.OutputEditing = true
	syncOutputPath(&model.Setup)

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if cmd != nil {
		t.Fatalf("expected no command, got %#v", cmd)
	}
	if updated.Setup.OutputEditing {
		t.Fatalf("expected editing to stop: %+v", updated.Setup)
	}
	if updated.Notice == nil || updated.Notice.Text != "Finished editing the output target." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestChooseOutputArrowMovesSelectionWhenNotEditing(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Cursor = 2

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	updated := next.(Model)
	if updated.Cursor != 1 {
		t.Fatalf("unexpected cursor: %d", updated.Cursor)
	}
}

func TestRecoveryTargetInspectionMovesToReviewForNewTarget(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.ActiveTargetRequest = 7
	model.Setup.OutputPath = "D:/Archives/disc.iso"

	next, _ := model.Update(RecoveryTargetInspectedMsg{
		RequestID: 7,
		Status: platform.RecoveryTargetStatus{
			OutputPath:     "D:/Archives/disc.iso",
			MapPath:        "D:/Archives/disc.drmap",
			CanStartNew:    true,
			RequiredBytes:  4096,
			AvailableBytes: 8192,
			SpaceKnown:     true,
			Detail:         "A new recovery will be created at this path.",
		},
	})
	updated := next.(Model)
	if updated.Page != PageReview {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageReview)
	}
	if updated.Setup.ActionLabel != "Start a new recovery" {
		t.Fatalf("unexpected action label: %q", updated.Setup.ActionLabel)
	}
	if updated.Setup.FreeSpace != "8.0 KiB free; need 4.0 KiB" {
		t.Fatalf("unexpected free-space summary: %q", updated.Setup.FreeSpace)
	}
}

func TestRecoveryTargetInspectionMovesToReviewForResumableTarget(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.ActiveTargetRequest = 8

	next, _ := model.Update(RecoveryTargetInspectedMsg{
		RequestID: 8,
		Status: platform.RecoveryTargetStatus{
			OutputPath:        "D:/Archives/disc.iso",
			MapPath:           "D:/Archives/disc.drmap",
			CanResume:         true,
			RecoveredSectors:  120,
			UnreadableSectors: 3,
			RequiredBytes:     4096,
			AvailableBytes:    8192,
			SpaceKnown:        true,
			Detail:            "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
		},
	})
	updated := next.(Model)
	if updated.Page != PageReview {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageReview)
	}
	if updated.Setup.ActionLabel != "Resume recovery" {
		t.Fatalf("unexpected action label: %q", updated.Setup.ActionLabel)
	}
	if !updated.Setup.ResumeReady || updated.Setup.ResumeMapPath != "D:/Archives/disc.drmap" {
		t.Fatalf("unexpected resume state: %+v", updated.Setup)
	}
	if updated.Setup.FreeSpace != "8.0 KiB free; need 4.0 KiB" {
		t.Fatalf("unexpected free-space summary: %q", updated.Setup.FreeSpace)
	}
}

func TestResumableJobsDiscoveryMovesToResumePage(t *testing.T) {
	model := NewModel()
	model.ActiveResumeRequest = 9

	next, _ := model.Update(ResumableJobsDiscoveredMsg{
		RequestID: 9,
		Jobs: []ResumableJobViewModel{{
			OutputPath: "D:/Archives/disc.iso",
			MapPath:    "D:/Archives/disc.drmap",
			Detail:     "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
		}},
	})
	updated := next.(Model)
	if updated.Page != PageResumeJobs {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageResumeJobs)
	}
	if len(updated.ResumeJobs) != 1 {
		t.Fatalf("unexpected resume jobs: %+v", updated.ResumeJobs)
	}
}

func TestProcessedMediaDiscoveryMovesToHistoryPage(t *testing.T) {
	model := NewModel()
	model.ActiveHistoryRequest = 10

	next, _ := model.Update(ProcessedMediaDiscoveredMsg{
		RequestID: 10,
		Items: []ProcessedMediaViewModel{{
			Title:     "archive-disc.iso",
			ImagePath: "D:/Archives/archive-disc.iso",
			Status:    "Saved with map",
		}},
	})
	updated := next.(Model)
	if updated.Page != PageHistory {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageHistory)
	}
	if len(updated.HistoryItems) != 1 {
		t.Fatalf("unexpected history items: %+v", updated.HistoryItems)
	}
}

func TestResumeJobsSelectMovesToReview(t *testing.T) {
	model := NewModel()
	model.Page = PageResumeJobs
	model.ResumeJobs = []ResumableJobViewModel{{
		OutputPath:        filepath.Join("D:/Archives", "disc.iso"),
		MapPath:           filepath.Join("D:/Archives", "disc.drmap"),
		RecoveredSectors:  120,
		UnreadableSectors: 3,
		Detail:            "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
	}}

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageReview {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageReview)
	}
	if updated.Setup.ActionLabel != "Resume recovery" || !updated.Setup.ResumeReady {
		t.Fatalf("unexpected setup state: %+v", updated.Setup)
	}
}

func TestHistorySelectOpensDetails(t *testing.T) {
	model := NewModel()
	model.Page = PageHistory
	model.HistoryItems = []ProcessedMediaViewModel{{
		Title:      "archive-disc.iso",
		ImagePath:  "D:/Archives/archive-disc.iso",
		MapPath:    "D:/Archives/archive-disc.drmap",
		Status:     "Saved with map",
		ModifiedAt: "2026-08-06 10:15",
		Detail:     "A recovery map exists, but it does not match the currently selected disc.",
	}}

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageDetails || updated.PreviousPage != PageHistory {
		t.Fatalf("unexpected page transition: %+v", updated)
	}
	if len(updated.Details.Lines) == 0 || updated.Details.Lines[0] != "Image: D:/Archives/archive-disc.iso" {
		t.Fatalf("unexpected details: %+v", updated.Details.Lines)
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

func TestSpaceDuringRecoveryMovesToPausingAndRequestsPause(t *testing.T) {
	model := NewModel()
	model.Page = PageRecovering

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	updated := next.(Model)
	if updated.Page != PagePausing {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PagePausing)
	}
	if !updated.Recovery.PausePending {
		t.Fatalf("expected pause pending state: %+v", updated.Recovery)
	}
	if updated.Notice == nil || updated.Notice.Text != "Pausing recovery after the current read completes." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
	requested := cmd().(EffectRequestedMsg)
	if requested.Kind != EffectPauseJob {
		t.Fatalf("unexpected pause effect: %+v", requested)
	}
}

func TestPausingIgnoresSelectUntilPauseCompletes(t *testing.T) {
	model := NewModel()
	model.Page = PagePausing
	model.Recovery.PausePending = true

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if cmd != nil {
		t.Fatalf("expected no command while pause is pending, got %#v", cmd)
	}
	if updated.Page != PagePausing {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PagePausing)
	}
}

func TestJobPausedMovesToPausedState(t *testing.T) {
	model := NewModel()
	model.Page = PagePausing

	next, _ := model.Update(JobPausedMsg{
		OutputPath:        "D:/Archives/archive-disc.iso",
		MapPath:           "D:/Archives/archive-disc.drmap",
		RecoveredSectors:  120,
		TotalSectors:      240,
		UnreadableSectors: 3,
	})
	updated := next.(Model)
	if updated.Page != PagePaused {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PagePaused)
	}
	if updated.Recovery.PausePending {
		t.Fatalf("expected pause pending to clear: %+v", updated.Recovery)
	}
	if updated.Notice == nil || updated.Notice.Text != "Progress saved. Continue recovery when you are ready." {
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

func TestSummaryPrimaryChoiceRetriesDeferredSectors(t *testing.T) {
	model := NewModel()
	model.Page = PageSummary
	model.Recovery = RecoveryViewModel{
		DeferredSectors: 64,
	}

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(Model)
	if updated.Page != PageRecovering {
		t.Fatalf("expected retry choice to return to recovery, got %v", updated.Page)
	}
	if cmd == nil {
		t.Fatal("expected retry choice to schedule resume effect")
	}
	msg := cmd()
	request, ok := msg.(EffectRequestedMsg)
	if !ok || request.Kind != EffectResumeJob {
		t.Fatalf("expected resume effect, got %#v", msg)
	}
}

func TestRecoveryTargetInspectionKeepsChooseOutputForOccupiedTarget(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.ActiveTargetRequest = 11

	next, _ := model.Update(RecoveryTargetInspectedMsg{
		RequestID: 11,
		Status: platform.RecoveryTargetStatus{
			OutputPath: "D:/Archives/disc.iso",
			MapPath:    "D:/Archives/disc.drmap",
			Detail:     "Output image D:/Archives/disc.iso already exists without D:/Archives/disc.drmap. Choose another output path.",
		},
	})
	updated := next.(Model)
	if updated.Page != PageChooseOutput {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseOutput)
	}
	if updated.Notice == nil || updated.Notice.Text != "Output image D:/Archives/disc.iso already exists without D:/Archives/disc.drmap. Choose another output path." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestRecoveryTargetInspectionBlocksTooSmallTargetBeforeReview(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.ActiveTargetRequest = 12

	next, _ := model.Update(RecoveryTargetInspectedMsg{
		RequestID: 12,
		Status: platform.RecoveryTargetStatus{
			OutputPath:     "D:/Archives/disc.iso",
			MapPath:        "D:/Archives/disc.drmap",
			RequiredBytes:  16384,
			AvailableBytes: 8192,
			SpaceKnown:     true,
			Detail:         "The selected output drive does not have enough free space for this image. Need 16.0 KiB and only 8.0 KiB are free. Choose another output path.",
		},
	})
	updated := next.(Model)
	if updated.Page != PageChooseOutput {
		t.Fatalf("unexpected page: got %v want %v", updated.Page, PageChooseOutput)
	}
	if updated.Setup.FreeSpace != "8.0 KiB free; need 16.0 KiB — choose another target" {
		t.Fatalf("unexpected free-space summary: %q", updated.Setup.FreeSpace)
	}
	if updated.Notice == nil || updated.Notice.Text != "The selected output drive does not have enough free space for this image. Need 16.0 KiB and only 8.0 KiB are free. Choose another output path." {
		t.Fatalf("unexpected notice: %+v", updated.Notice)
	}
}

func TestChooseOutputEditingClearsPreviousTargetSpaceStatus(t *testing.T) {
	model := NewModel()
	model.Page = PageChooseOutput
	model.Setup.OutputDirectory = "D:/Archives"
	model.Setup.OutputFileName = "disc.iso"
	model.Setup.ActiveOutputField = OutputFieldFileName
	model.Setup.OutputEditing = true
	model.Setup.FreeSpace = "8.0 GiB free; need 4.0 GiB"
	syncOutputPath(&model.Setup)

	next, _ := model.Update(tea.KeyPressMsg{Text: "2"})
	updated := next.(Model)
	if updated.Setup.FreeSpace != "Check the selected target to see free space and required size" {
		t.Fatalf("unexpected free-space reset: %q", updated.Setup.FreeSpace)
	}
}
