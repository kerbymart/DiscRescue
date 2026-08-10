package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"discrescue/internal/catalog"
)

func TestEveryPageFitsSupportedTerminalSizes(t *testing.T) {
	pages := []Page{
		PageDiscover, PageNoDrives, PageDiscoveryError, PageChooseDrive,
		PageInspectingMedia, PagePriorProcessing, PageChooseAction, PageChooseOutput,
		PageReview, PageRecovering, PagePausing, PagePaused, PageStopConfirm,
		PageSummary, PageResumeJobs, PageHistory, PageDetails, PageAdvanced, PageAbout,
	}
	sizes := []struct{ width, height int }{{120, 36}, {80, 24}, {60, 18}, {40, 12}}
	for _, size := range sizes {
		for _, page := range pages {
			model := representativeViewModel(page)
			updated, _ := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
			content := strings.TrimSuffix(updated.View().Content, "\n")
			if got := lipgloss.Width(content); got > size.width {
				t.Errorf("page %d at %dx%d is %d columns wide", page, size.width, size.height, got)
			}
			if got := lipgloss.Height(content); got > size.height {
				t.Errorf("page %d at %dx%d is %d rows tall", page, size.width, size.height, got)
			}
		}
	}
}

func TestResponsiveShellHandlesUnicodeAndMonochrome(t *testing.T) {
	model := representativeViewModel(PageDetails)
	model.Monochrome = true
	model.Details = DetailsViewModel{Lines: []string{
		"Drive: /archive/光学ディスク/семейные-фильмы-第七巻",
		"Media: Café · naïve · é · 漢字",
		strings.Repeat("long technical detail ", 12),
	}}
	for _, size := range []struct{ width, height int }{{120, 36}, {80, 24}, {60, 18}, {40, 12}} {
		updated, _ := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		content := strings.TrimSuffix(updated.View().Content, "\n")
		if got := lipgloss.Width(content); got > size.width {
			t.Errorf("unicode monochrome view at %dx%d is %d columns wide", size.width, size.height, got)
		}
		if got := lipgloss.Height(content); got > size.height {
			t.Errorf("unicode monochrome view at %dx%d is %d rows tall", size.width, size.height, got)
		}
	}
}

func representativeViewModel(page Page) Model {
	model := NewModel()
	model.Page = page
	model.Devices = []DeviceSummary{{DisplayName: "Optical drive", Path: "/dev/sr0", Status: "disc present"}}
	model.SelectedDrive = model.Devices[0]
	model.Identity = ContentIdentityViewModel{Summary: "DVD-ROM", Detail: "DVD-ROM · 4.38 GiB · UDF"}
	model.MediaRecoverable = true
	model.PriorView = PriorProcessingViewModel{
		Kind: PriorProcessingStrongResumable, Title: "Matching contents have unfinished work",
		Body: []string{"A durable recovery map is available."}, Options: []string{"Resume recovery", "Start another capture", "Back"},
	}
	model.Setup.OutputDirectory = "/archive"
	model.Setup.OutputFileName = "family-movies.iso"
	model.Setup.OutputPath = "/archive/family-movies.iso"
	model.Setup.FreeSpace = "128 GiB available · 4.38 GiB required"
	model.DirectoryInput.SetValue(model.Setup.OutputDirectory)
	model.FileNameInput.SetValue(model.Setup.OutputFileName)
	model.Recovery = RecoveryViewModel{
		Phase: "Fast acquisition", Status: "Scanning forward; difficult ranges are deferred.",
		ScannedSectors: 1526100, RecoveredSectors: 1518124, DeferredSectors: 7904,
		UnreadableSectors: 72, TotalSectors: 2289072, OutputPath: model.Setup.OutputPath,
		Elapsed: "18m 42s", Remaining: "1.5 GiB remaining", ETA: "about 12m left", Throughput: "6.8 MiB/s",
		LastIssue: []string{"Read difficulty near sector 1,498,112; saved for a later pass."},
	}
	model.Summary = JobSummary{ImagePath: model.Setup.OutputPath, MapPath: "/archive/family-movies.drmap", Duration: "31m 04s"}
	model.ResumeJobs = []ResumableJobViewModel{{OutputPath: model.Setup.OutputPath, MapPath: model.Summary.MapPath, Detail: "Safe to resume."}}
	model.HistoryItems = []ProcessedMediaViewModel{{Title: "Family movies", ImagePath: model.Setup.OutputPath, Status: "Incomplete"}}
	model.Details = DetailsViewModel{Lines: []string{"Drive: /dev/sr0", "Worker: active", "Map: durable"}}
	return model
}

func TestViewTooSmallKeepsAlternateScreen(t *testing.T) {
	model := NewModel()
	model.Width = 39
	model.Height = 11

	rendered := model.View()
	view := rendered.Content
	if !strings.Contains(view, "Resize to at least 40x12") {
		t.Fatalf("unexpected small-window view: %q", view)
	}
	if !rendered.AltScreen {
		t.Fatal("expected small-window view to remain in the application alternate screen")
	}
}

func TestViewChooseDriveUsesApplicationAlternateScreen(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageChooseDrive

	if view := model.View(); !view.AltScreen {
		t.Fatal("expected drive selection to use the application alternate screen")
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

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Choose a drive") {
		t.Fatalf("expected choose-drive title, got %q", view)
	}
	if !strings.Contains(view, "> DVD Drive") {
		t.Fatalf("expected focused device row, got %q", view)
	}
	if !strings.Contains(view, "AVAILABLE DRIVES") || !strings.Contains(view, "╭") {
		t.Fatalf("expected a focused drive card, got %q", view)
	}
}

func TestViewDiscoverShowsNonInteractiveStartupCopy(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Finding usable optical drives") {
		t.Fatalf("expected discovery title, got %q", view)
	}
	if !strings.Contains(view, "Looking for optical drives") || !strings.Contains(view, "This usually takes only a moment.") {
		t.Fatalf("expected discovery body copy, got %q", view)
	}
	if strings.Contains(view, "j/k move") || strings.Contains(view, "esc back") {
		t.Fatalf("expected startup view to avoid selection controls, got %q", view)
	}
	if strings.Contains(view, "Status: Finding usable optical drives.") {
		t.Fatalf("expected startup view to avoid duplicated status text, got %q", view)
	}
	if !strings.Contains(view, "retry discovery") || !strings.Contains(view, "q") || !strings.Contains(view, "quit") {
		t.Fatalf("expected startup view to keep quit available, got %q", view)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Resume unfinished recovery") {
		t.Fatalf("expected resume page title, got %q", view)
	}
	if !strings.Contains(view, "> D:/Archives/archive-disc.iso") {
		t.Fatalf("expected resumable job row, got %q", view)
	}
	if !strings.Contains(view, "Resume recovery from 120 recovered sectors and 3") ||
		!strings.Contains(view, "unreadable sectors.") {
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Folder") || !strings.Contains(view, "File name") {
		t.Fatalf("expected explicit output fields, got %q", view)
	}
	if !strings.Contains(view, "Archive-Disc-Volume-07.iso") {
		t.Fatalf("expected wrapped path tail, got %q", view)
	}
	if !strings.Contains(view, "Continue with this target") {
		t.Fatalf("expected a clear primary output action, got %q", view)
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
		OutputPath:        "D:/Archives/archive-disc.iso",
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
	if !strings.Contains(view.Content, "Recovering archive-disc.iso") {
		t.Fatalf("expected recovery title to include output file name, got %q", view.Content)
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

func TestFullRecoveryDashboardUsesCoverageAndMetricZones(t *testing.T) {
	model := representativeViewModel(PageRecovering)
	model.Width = 120
	model.Height = 36

	view := ansi.Strip(model.View().Content)
	for _, want := range []string{"Coverage", "Recovered", "Deferred", "Unreadable", "Remaining", "Rate", "Elapsed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("full recovery dashboard missing %q: %q", want, view)
		}
	}
	if !strings.Contains(view, "├") || !strings.Contains(view, "┼") {
		t.Fatalf("full recovery dashboard should use one divided status panel: %q", view)
	}
}

func TestRecoveryDashboardUsesOrganizedCardsAtEightyByTwentyFour(t *testing.T) {
	model := representativeViewModel(PageRecovering)
	model.Width = 80
	model.Height = 24

	view := ansi.Strip(model.View().Content)
	for _, want := range []string{"Coverage", "Recovered", "Deferred", "Unreadable", "Remaining"} {
		if !strings.Contains(view, want) {
			t.Fatalf("80x24 recovery dashboard missing %q: %q", want, view)
		}
	}
	if !strings.Contains(view, "├") || !strings.Contains(view, "┼") {
		t.Fatalf("80x24 recovery dashboard should use one divided status panel: %q", view)
	}
	if strings.Contains(view, "Coverage  66%") {
		t.Fatalf("80x24 recovery dashboard still uses the legacy flat progress layout: %q", view)
	}
	if got := lipgloss.Height(strings.TrimSuffix(model.View().Content, "\n")); got > model.Height {
		t.Fatalf("80x24 recovery dashboard is %d rows tall", got)
	}
}

func TestViewRecoveringUsesCompactLayoutAtFortyByTwelve(t *testing.T) {
	model := NewModel()
	model.Width = 40
	model.Height = 12
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		Phase:             "Reading healthy areas",
		OutputPath:        "D:/Archives/archive-disc.iso",
		RecoveredSectors:  1554208,
		TotalSectors:      2295104,
		UnreadableSectors: 37,
		Elapsed:           "2m11s",
		Remaining:         "1.42 GiB of 4.38 GiB remaining",
		ETA:               "about 7 minutes",
		LastIssue:         []string{"Last issue: sector 1,891,840 could not be read.", "It will be tried again during the recovery pass."},
	}

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Recovering") {
		t.Fatalf("expected recovery title, got %q", view)
	}
	if !strings.Contains(view, "Coverage") || !strings.Contains(view, "67%") {
		t.Fatalf("expected compact coverage rail, got %q", view)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "█") && !strings.Contains(view, "░") {
		t.Fatalf("expected unicode progress bar, got %q", view)
	}
	if strings.Contains(view, "â–ˆ") || strings.Contains(view, "â–‘") {
		t.Fatalf("expected valid unicode progress bar glyphs, got %q", view)
	}
}

func TestRecoveryProgressShowsThickOverallCoverage(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		RecoveredSectors:  40,
		DeferredSectors:   20,
		UnreadableSectors: 10,
		ScannedSectors:    70,
		TotalSectors:      100,
	}

	progress := ansi.Strip(recoveryProgressLine(model, 70, layoutMedium))
	if strings.Count(progress, "\n") != 1 {
		t.Fatalf("expected compact one-row progress rail and coverage label, got %q", progress)
	}
	if !strings.Contains(progress, "Coverage  70%   70 / 100 sectors") {
		t.Fatalf("expected overall coverage label, got %q", progress)
	}
}

func TestSegmentedRecoveryBarUsesDistinctMonochromeStates(t *testing.T) {
	model := NewModel()
	model.Monochrome = true
	model.Recovery = RecoveryViewModel{
		RecoveredSectors:  10,
		DeferredSectors:   4,
		UnreadableSectors: 2,
		TotalSectors:      20,
	}

	if got, want := segmentedRecoveryBar(model, 20), "[==========~~~~xx....]"; got != want {
		t.Fatalf("segmented monochrome rail = %q, want %q", got, want)
	}
}

func TestViewRecoveringShowsEstimatingTextWhenETANotReady(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageRecovering
	model.Recovery = RecoveryViewModel{
		OutputPath:       "D:/Archives/archive-disc.iso",
		RecoveredSectors: 120,
		TotalSectors:     240,
		Remaining:        "1.0 GiB remaining",
	}

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "ETA estimating") {
		t.Fatalf("expected ETA fallback copy, got %q", view)
	}
}

func TestViewPausingShowsPendingPauseLanguage(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PagePausing
	model.Recovery = RecoveryViewModel{PausePending: true}

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Pausing recovery") {
		t.Fatalf("expected pausing title, got %q", view)
	}
	if !strings.Contains(view, "Pause requested. Waiting for the current drive request to") || !strings.Contains(view, "safely.") {
		t.Fatalf("expected truthful pause-pending summary, got %q", view)
	}
	if strings.Contains(view, "Continue recovery") {
		t.Fatalf("did not expect resume action while pause is still pending, got %q", view)
	}
}

func TestViewStoppingShowsCheckpointAndForceStopGuidance(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PagePausing
	model.Recovery = RecoveryViewModel{StopPending: true, ForceStopAvailable: true}

	view := ansi.Strip(model.View().Content)
	for _, want := range []string{"Saving progress and stopping", "Stop requested.", "Press x to force-stop"} {
		if !strings.Contains(view, want) {
			t.Fatalf("stopping view missing %q: %q", want, view)
		}
	}
}

func TestViewPausedShowsSafeResumeLanguage(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PagePaused

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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
		CatalogStatus: catalog.CatalogWriteStatus{State: catalog.CatalogWriteRecorded},
	}
	model.Recovery = RecoveryViewModel{
		Status:            "Recovery finished with unreadable sectors",
		OutputPath:        "D:/Archives/archive-disc.iso",
		RecoveredSectors:  2295067,
		TotalSectors:      2295104,
		UnreadableSectors: 37,
	}

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "37 sectors could not be recovered.") {
		t.Fatalf("expected incomplete-result explanation, got %q", view)
	}
	if !strings.Contains(view, "> Retry unreadable sectors") {
		t.Fatalf("expected retry-first choice, got %q", view)
	}
	if !strings.Contains(view, "D:/Archives/archive-disc.drmap") {
		t.Fatalf("expected explicit map path, got %q", view)
	}
	if !strings.Contains(view, "Choose another drive") {
		t.Fatalf("expected real follow-up action, got %q", view)
	}
}

func TestViewIncompleteSummaryOmitsHistoryWhenCatalogWriteWasNotAttempted(t *testing.T) {
	model := NewModel()
	model.Width = 80
	model.Height = 24
	model.Page = PageSummary
	model.Summary = JobSummary{
		ImagePath: "D:/Archives/archive-disc.iso",
		MapPath:   "D:/Archives/archive-disc.drmap",
	}
	model.Recovery = RecoveryViewModel{
		Status:            "Recovery finished with unreadable sectors",
		OutputPath:        "D:/Archives/archive-disc.iso",
		RecoveredSectors:  2295067,
		TotalSectors:      2295104,
		UnreadableSectors: 37,
	}

	view := ansi.Strip(model.View().Content)
	if strings.Contains(view, "Recorded in local processed-media catalog") {
		t.Fatalf("summary invented a catalog write: %q", view)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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

	view := ansi.Strip(model.View().Content)
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
