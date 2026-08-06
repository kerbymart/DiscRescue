package app

import (
	"strings"
	"testing"
)

func TestViewTooSmall(t *testing.T) {
	model := NewModel()
	model.Width = 39
	model.Height = 11

	rendered := model.View()
	view := rendered.Content
	if !strings.Contains(view, "Resize to at least 40x12") {
		t.Fatalf("unexpected small-window view: %q", view)
	}
	if rendered.AltScreen {
		t.Fatal("expected non-recovery small-window view to stay out of the alternate screen")
	}
}

func TestViewTooSmallKeepsRecoveryAltScreenAndActivityNotice(t *testing.T) {
	model := NewModel()
	model.Width = 39
	model.Height = 11
	model.Page = PageRecovering

	rendered := model.View()
	if !rendered.AltScreen {
		t.Fatal("expected recovery view to preserve alternate-screen state while too small")
	}
	if !strings.Contains(rendered.Content, "Recovery continues while you resize the window.") {
		t.Fatalf("expected recovery resize notice, got %q", rendered.Content)
	}
}

func TestViewTooSmallKeepsPausingAltScreenAndActivityNotice(t *testing.T) {
	model := NewModel()
	model.Width = 39
	model.Height = 11
	model.Page = PagePausing

	rendered := model.View()
	if !rendered.AltScreen {
		t.Fatal("expected pausing view to preserve alternate-screen state while too small")
	}
	if !strings.Contains(rendered.Content, "Recovery continues while you resize the window.") {
		t.Fatalf("expected recovery resize notice, got %q", rendered.Content)
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
	model.MediaRecoverable = true
	model.PriorView = PriorProcessingViewModel{
		Kind:        PriorProcessingNone,
		HistoryLine: "Checking this computer for matching saved work.",
	}

	view := model.View().Content
	if !strings.Contains(view, "Checking this computer for matching saved work.") {
		t.Fatalf("expected history line, got %q", view)
	}
	if !strings.Contains(view, "Disc: DVD-ROM, 4.38 GiB") {
		t.Fatalf("expected disc summary, got %q", view)
	}
	if !strings.Contains(view, "> Start a new recovery") {
		t.Fatalf("expected start action, got %q", view)
	}
	if !strings.Contains(view, "Resume an unfinished recovery") {
		t.Fatalf("expected resume action, got %q", view)
	}
	if !strings.Contains(view, "Browse processed media") {
		t.Fatalf("expected browse-history action, got %q", view)
	}
}

func TestViewReviewUsesCompactUserFacingSummary(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageReview
	model.SelectedDrive = DeviceSummary{DisplayName: "/dev/sr0"}
	model.Identity = ContentIdentityViewModel{Detail: "DVD-ROM, 4.38 GiB"}
	model.Setup.OutputPath = "~/Images/archive-disc.iso"

	view := model.View().Content
	if !strings.Contains(view, "Drive") || !strings.Contains(view, "/dev/sr0") {
		t.Fatalf("expected selected drive summary, got %q", view)
	}
	if !strings.Contains(view, "> Start a new recovery") {
		t.Fatalf("expected safe default review action, got %q", view)
	}
	if strings.Contains(strings.ToLower(view), "cluster") || strings.Contains(strings.ToLower(view), "retry budget") {
		t.Fatalf("unexpected advanced raw values in review page: %q", view)
	}
}

func TestViewReviewShowsResumeTargetWhenAvailable(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageReview
	model.SelectedDrive = DeviceSummary{DisplayName: "Optical drive E:"}
	model.Identity = ContentIdentityViewModel{Detail: "DVD-ROM, 4.38 GiB"}
	model.Setup.OutputPath = "D:/Archives/archive-disc.iso"
	model.Setup.ActionLabel = "Resume recovery"
	model.Setup.ResumeReady = true
	model.Setup.ResumeMapPath = "D:/Archives/archive-disc.drmap"
	model.Setup.ResumeDetail = "Resume recovery from 120 recovered sectors and 3 unreadable sectors."

	view := model.View().Content
	if !strings.Contains(view, "Map") || !strings.Contains(view, "archive-disc.drmap") {
		t.Fatalf("expected resume map details, got %q", view)
	}
	if !strings.Contains(view, "> Resume recovery") {
		t.Fatalf("expected resume action, got %q", view)
	}
	if !strings.Contains(view, "Resume recovery from 120 recovered sectors and 3 unreadable sectors.") {
		t.Fatalf("expected resume detail, got %q", view)
	}
}

func TestViewResumeJobsShowsSavedRecoveries(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageResumeJobs
	model.ResumeJobs = []ResumableJobViewModel{{
		OutputPath: "D:/Archives/archive-disc.iso",
		MapPath:    "D:/Archives/archive-disc.drmap",
		Detail:     "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
	}}

	view := model.View().Content
	if !strings.Contains(view, "Resume unfinished recovery") {
		t.Fatalf("expected resume page title, got %q", view)
	}
	if !strings.Contains(view, "> D:/Archives/archive-disc.iso") {
		t.Fatalf("expected resumable job row, got %q", view)
	}
	if !strings.Contains(view, "Resume recovery from 120 recovered sectors and 3 unreadable") ||
		!strings.Contains(view, "sectors.") {
		t.Fatalf("expected resume details, got %q", view)
	}
}

func TestViewHistoryShowsProcessedMediaList(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageHistory
	model.HistoryItems = []ProcessedMediaViewModel{{
		Title:      "archive-disc.iso",
		ImagePath:  "D:/Archives/archive-disc.iso",
		MapPath:    "D:/Archives/archive-disc.drmap",
		Status:     "Resumable",
		ModifiedAt: "2026-08-06 10:15",
		Detail:     "Resume recovery from 120 recovered sectors and 3 unreadable sectors.",
	}}

	view := model.View().Content
	if !strings.Contains(view, "Browse processed media") {
		t.Fatalf("expected history page title, got %q", view)
	}
	if !strings.Contains(view, "> archive-disc.iso") {
		t.Fatalf("expected history item row, got %q", view)
	}
	if !strings.Contains(view, "Status") || !strings.Contains(view, "Resumable") {
		t.Fatalf("expected item status details, got %q", view)
	}
}

func TestViewOutputWrapsLongPathInsteadOfTruncating(t *testing.T) {
	model := NewModel()
	model.Width = 60
	model.Height = 18
	model.Page = PageChooseOutput
	model.Setup.OutputDirectory = "D:/Archives/OpticalDiscCaptures/Very-Long-Collection-Name"
	model.Setup.OutputFileName = "Archive-Disc-Volume-07.iso"
	model.Setup.ActiveOutputField = OutputFieldFileName
	syncOutputPath(&model.Setup)

	view := model.View().Content
	if !strings.Contains(view, "Folder") || !strings.Contains(view, "File name") {
		t.Fatalf("expected explicit output fields, got %q", view)
	}
	if !strings.Contains(view, "Archive-Disc-Volume-07.iso") {
		t.Fatalf("expected wrapped path tail, got %q", view)
	}
	if !strings.Contains(view, "Choose where to save the recovery image.") {
		t.Fatalf("expected clearer output guidance, got %q", view)
	}
	if !strings.Contains(view, "Full path") {
		t.Fatalf("expected combined path preview, got %q", view)
	}
}

func TestViewRecoveringUsesAltScreenAndNoTelemetryTable(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		Phase:             "Reading healthy areas",
		RecoveredSectors:  1554208,
		TotalSectors:      2295104,
		UnreadableSectors: 37,
		Elapsed:           "2m11s",
		Remaining:         "1.42 GiB of 4.38 GiB remaining",
		ETA:               "about 7 minutes",
		Throughput:        "18.2 MiB/s",
		LastIssue:         []string{"Last issue: sector 1,891,840 could not be read.", "It will be tried again during the recovery pass."},
		Status:            "Reading difficult areas.",
	}

	view := model.View()
	if !view.AltScreen {
		t.Fatal("expected recovering page to use the alternate screen")
	}
	if !strings.Contains(view.Content, "Reading healthy areas") || !strings.Contains(view.Content, "about 7 minutes") {
		t.Fatalf("expected recovery summary fields, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "Rate") || !strings.Contains(view.Content, "Elapsed") {
		t.Fatalf("expected richer progress details, got %q", view.Content)
	}
	if strings.Contains(strings.ToLower(view.Content), "chart") {
		t.Fatalf("unexpected telemetry content: %q", view.Content)
	}
}

func TestViewRecoveringUsesCompactLayoutAtFortyByTwelve(t *testing.T) {
	model := NewModel()
	model.Width = 40
	model.Height = 12
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		Phase:             "Reading healthy areas",
		RecoveredSectors:  1554208,
		TotalSectors:      2295104,
		UnreadableSectors: 37,
		Elapsed:           "2m11s",
		Remaining:         "1.42 GiB of 4.38 GiB remaining",
		ETA:               "about 7 minutes",
		LastIssue:         []string{"Last issue: sector 1,891,840 could not be read.", "It will be tried again during the recovery pass."},
	}

	view := model.View().Content
	if !strings.Contains(view, "[") || !strings.Contains(view, "]") {
		t.Fatalf("expected compact progress bar, got %q", view)
	}
	if strings.Contains(view, "Last issue: sector 1,891,840 could not be read.") {
		t.Fatalf("expected compact layout to omit issue detail, got %q", view)
	}
	if strings.Contains(view, "about 7 minutes") {
		t.Fatalf("expected compact layout to omit ETA detail, got %q", view)
	}
}

func TestViewRecoveringUsesMonochromeSafeProgressBar(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageRecovering
	model.Monochrome = true
	model.Recovery = RecoveryViewModel{
		RecoveredSectors: 120,
		TotalSectors:     240,
	}

	view := model.View().Content
	if !strings.Contains(view, "[========") && !strings.Contains(view, "[====") {
		t.Fatalf("expected monochrome-safe progress bar, got %q", view)
	}
	if strings.Contains(view, "█") || strings.Contains(view, "░") {
		t.Fatalf("unexpected non-monochrome glyphs in progress bar: %q", view)
	}
}

func TestViewRecoveringUsesUnicodeProgressBarWhenAllowed(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		RecoveredSectors: 120,
		TotalSectors:     240,
	}

	view := model.View().Content
	if !strings.Contains(view, "█") && !strings.Contains(view, "░") {
		t.Fatalf("expected unicode progress bar, got %q", view)
	}
}

func TestViewPausingShowsPendingPauseLanguage(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PagePausing
	model.Recovery = RecoveryViewModel{PausePending: true}

	view := model.View().Content
	if !strings.Contains(view, "Pausing recovery") {
		t.Fatalf("expected pausing title, got %q", view)
	}
	if !strings.Contains(view, "Pause requested. Waiting for the current drive request to finish safely.") {
		t.Fatalf("expected truthful pause-pending summary, got %q", view)
	}
	if strings.Contains(view, "Continue recovery") {
		t.Fatalf("did not expect resume action while pause is still pending, got %q", view)
	}
}

func TestViewPausedShowsSafeResumeLanguage(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PagePaused

	view := model.View().Content
	if !strings.Contains(view, "Recovery paused") {
		t.Fatalf("expected paused title, got %q", view)
	}
	if !strings.Contains(view, "The current image and recovery map are safe to resume.") {
		t.Fatalf("expected paused summary, got %q", view)
	}
	if !strings.Contains(view, "> Continue recovery") {
		t.Fatalf("expected resume action, got %q", view)
	}
}

func TestViewStopConfirmationPlacesImmediateTerminationLast(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageStopConfirm

	view := model.View().Content
	if !strings.Contains(view, "> Save progress and stop") {
		t.Fatalf("expected safe stop default, got %q", view)
	}
	if strings.Contains(view, "Stop worker immediately") {
		t.Fatalf("did not expect fake immediate-stop option, got %q", view)
	}
}

func TestViewIncompleteSummaryAvoidsCleanSuccessTreatment(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageSummary
	model.Summary = JobSummary{
		ImagePath:     "D:/Archives/archive-disc.iso",
		MapPath:       "D:/Archives/archive-disc.drmap",
		CatalogStatus: "Recorded in local processed-media catalog",
	}
	model.Recovery = RecoveryViewModel{
		Status:            "Recovery finished with unreadable sectors",
		OutputPath:        "D:/Archives/archive-disc.iso",
		RecoveredSectors:  2295067,
		TotalSectors:      2295104,
		UnreadableSectors: 37,
	}

	view := model.View().Content
	if !strings.Contains(view, "37 sectors could not be recovered.") {
		t.Fatalf("expected incomplete-result explanation, got %q", view)
	}
	if !strings.Contains(view, "> Exit") {
		t.Fatalf("expected exit-first choice, got %q", view)
	}
	if !strings.Contains(view, "D:/Archives/archive-disc.drmap") {
		t.Fatalf("expected explicit map path, got %q", view)
	}
	if !strings.Contains(view, "Choose another drive") {
		t.Fatalf("expected real follow-up action, got %q", view)
	}
}

func TestViewCompleteSummaryShowsDuration(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageSummary
	model.Summary = JobSummary{
		Outcome:          "Recovery complete",
		ImagePath:        "D:/Archives/archive-disc.iso",
		RecoveredSectors: 2295104,
		TotalSectors:     2295104,
		Duration:         "31 minutes",
	}
	model.Recovery = RecoveryViewModel{
		Status:           "Recovery complete",
		OutputPath:       "D:/Archives/archive-disc.iso",
		RecoveredSectors: 2295104,
		TotalSectors:     2295104,
	}

	view := model.View().Content
	if !strings.Contains(view, "Duration   31 minutes") {
		t.Fatalf("expected duration in completion summary, got %q", view)
	}
	if !strings.Contains(view, "> Exit") {
		t.Fatalf("expected exit-first completion action, got %q", view)
	}
	if !strings.Contains(view, "Choose another drive") {
		t.Fatalf("expected choose-another-drive action, got %q", view)
	}
}

func TestViewCompactSummaryKeepsEssentialFieldsOnly(t *testing.T) {
	model := NewModel()
	model.Width = 40
	model.Height = 12
	model.Page = PageSummary
	model.Summary = JobSummary{
		Outcome:          "Recovery complete",
		ImagePath:        "D:/Archives/archive-disc.iso",
		RecoveredSectors: 2295104,
		TotalSectors:     2295104,
		Duration:         "31 minutes",
	}
	model.Recovery = RecoveryViewModel{
		Status:           "Recovery complete",
		OutputPath:       "D:/Archives/archive-disc.iso",
		RecoveredSectors: 2295104,
		TotalSectors:     2295104,
	}

	view := model.View().Content
	if !strings.Contains(view, "Image") {
		t.Fatalf("expected essential summary fields, got %q", view)
	}
	if strings.Contains(view, "Duration   31 minutes") {
		t.Fatalf("expected compact summary to omit duration detail, got %q", view)
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

func TestViewDetailsFromRecoveryUsesCurrentRecoveryDetails(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageDetails
	model.PreviousPage = PageRecovering
	model.SelectedDrive = DeviceSummary{DisplayName: "Optical drive E:", Path: "E:"}
	model.Identity = ContentIdentityViewModel{Detail: "DVD-ROM, 4.38 GiB"}
	model.Recovery = RecoveryViewModel{
		Phase:             "Reading optical sectors",
		Status:            "Reading sectors from the selected optical drive.",
		OutputPath:        "D:/Archives/disc.iso",
		RecoveredSectors:  120,
		UnreadableSectors: 3,
		Remaining:         "1.0 GiB remaining",
	}
	model.Summary = JobSummary{NextAction: "Fix the reported problem or choose a different output target."}
	model.Details = DetailsViewModel{Lines: []string{"Image: stale.iso", "Next step: stale summary text"}}

	view := model.View().Content
	if !strings.Contains(view, "Drive: Optical drive E:") {
		t.Fatalf("expected current recovery drive details, got %q", view)
	}
	if !strings.Contains(view, "Output: D:/Archives/disc.iso") {
		t.Fatalf("expected current recovery output, got %q", view)
	}
	if strings.Contains(view, "Next step: stale summary text") {
		t.Fatalf("did not expect stale summary details, got %q", view)
	}
}

func TestViewAboutShowsBuildMetadata(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageAbout

	view := model.View().Content
	if !strings.Contains(view, "Version dev") {
		t.Fatalf("expected default build version, got %q", view)
	}
	if !strings.Contains(view, "Commit unknown") {
		t.Fatalf("expected default build commit, got %q", view)
	}
	if !strings.Contains(view, "Build date unknown") {
		t.Fatalf("expected default build date, got %q", view)
	}
}
