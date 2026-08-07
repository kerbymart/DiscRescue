package app

import (
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
)

type Page uint8

const (
	PageDiscover Page = iota
	PageNoDrives
	PageDiscoveryError
	PageChooseDrive
	PageInspectingMedia
	PagePriorProcessing
	PageChooseAction
	PageChooseOutput
	PageReview
	PageRecovering
	PagePausing
	PagePaused
	PageStopConfirm
	PageSummary
	PageResumeJobs
	PageHistory
	PageDetails
	PageAdvanced
	PageAbout
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type DeviceSummary struct {
	Path        string
	DisplayName string
	Status      string
}

type OutputField uint8

const (
	OutputFieldDirectory OutputField = iota
	OutputFieldFileName
)

type JobSetupModel struct {
	ActionLabel       string
	OutputPath        string
	OutputDirectory   string
	OutputFileName    string
	ActiveOutputField OutputField
	OutputEditing     bool
	DefaultPath       string
	OutputFormat      string
	FreeSpace         string
	MethodLabel       string
	CopyLabel         string
	ResumeReady       bool
	ResumeMapPath     string
	ResumeDetail      string
}

type ContentIdentityViewModel struct {
	Summary string
	Detail  string
}

type PriorProcessingRecord struct {
	Title  string
	Detail string
}

type ProcessedMediaViewModel struct {
	Title      string
	ImagePath  string
	MapPath    string
	Status     string
	ModifiedAt string
	Detail     string
}

type ResumableJobViewModel struct {
	OutputPath        string
	MapPath           string
	RecoveredSectors  uint64
	DeferredSectors   uint64
	UnreadableSectors uint64
	Detail            string
}

type PriorProcessingKind string

const (
	PriorProcessingNone            PriorProcessingKind = "none"
	PriorProcessingStrongCompleted PriorProcessingKind = "strong_completed"
	PriorProcessingStrongResumable PriorProcessingKind = "strong_resumable"
	PriorProcessingProbable        PriorProcessingKind = "probable"
	PriorProcessingIndeterminate   PriorProcessingKind = "indeterminate"
)

type PriorProcessingViewModel struct {
	Kind              PriorProcessingKind
	Title             string
	Body              []string
	Options           []string
	HistoryLine       string
	ImagePath         string
	CopyLabel         string
	LastSaved         string
	Recovered         string
	DeferredSectors   string
	UnreadableSectors string
}

type RecoveryViewModel struct {
	Phase             string
	ScannedSectors    uint64
	RecoveredSectors  uint64
	DeferredSectors   uint64
	TotalSectors      uint64
	UnreadableSectors uint64
	Status            string
	OutputPath        string
	Elapsed           string
	Remaining         string
	ETA               string
	Throughput        string
	LastIssue         []string
	PausePending      bool
}

type DetailsViewModel struct {
	Lines []string
}

type DialogModel struct {
	Title   string
	Body    string
	Options []string
	Cursor  int
}

type NoticeModel struct {
	Text     string
	Severity Severity
}

type ProgressSnapshot struct {
	Phase             string
	ScannedSectors    uint64
	RecoveredSectors  uint64
	DeferredSectors   uint64
	TotalSectors      uint64
	UnreadableSectors uint64
	Status            string
	Elapsed           string
	Remaining         string
	ETA               string
	Throughput        string
	LastIssue         []string
	OutputPath        string
	PausePending      bool
}

type JobSummary struct {
	Outcome           string
	ImagePath         string
	MapPath           string
	NextAction        string
	UnresolvedSectors uint64
	DeferredSectors   uint64
	RecoveredSectors  uint64
	TotalSectors      uint64
	Duration          string
	CatalogStatus     string
}

type Model struct {
	Page                    Page
	PreviousPage            Page
	Width                   int
	Height                  int
	Devices                 []DeviceSummary
	SelectedDrive           DeviceSummary
	Cursor                  int
	Setup                   JobSetupModel
	Identity                ContentIdentityViewModel
	MediaFileSystem         string
	MediaVolumeLabel        string
	MediaLogicalSectorSize  uint32
	MediaCapacitySectors    uint64
	MediaRecoverable        bool
	MediaRecoverabilityNote string
	PriorView               PriorProcessingViewModel
	PriorRecords            []PriorProcessingRecord
	HistoryItems            []ProcessedMediaViewModel
	ResumeJobs              []ResumableJobViewModel
	Recovery                RecoveryViewModel
	Summary                 JobSummary
	Details                 DetailsViewModel
	Dialog                  *DialogModel
	Notice                  *NoticeModel
	LastError               error
	Quitting                bool
	Monochrome              bool
	NextRequestID           int
	ActiveDiscoveryRequest  int
	ActiveMediaRequest      int
	ActiveLookupRequest     int
	ActiveTargetRequest     int
	ActiveResumeRequest     int
	ActiveHistoryRequest    int
	DirectoryInput          textinput.Model
	FileNameInput           textinput.Model
	DetailsViewport         viewport.Model
	DriveList               list.Model
	ActionList              list.Model
	ResumeList              list.Model
	HistoryList             list.Model
	LoadingSpinner          spinner.Model
}

func NewModel() Model {
	directoryInput := textinput.New()
	directoryInput.Prompt = ""
	directoryInput.CharLimit = 4096
	directoryInput.SetValue(".")
	fileNameInput := textinput.New()
	fileNameInput.Prompt = ""
	fileNameInput.CharLimit = 255
	detailsViewport := viewport.New()
	driveList := newCompactList("Choose a drive", nil)
	actionList := newCompactList("What do you want to do?", choiceItems([]string{"Start a new recovery", "Resume an unfinished recovery", "Browse processed media", "Choose another drive"}))
	resumeList := newCompactList("Resume unfinished recovery", nil)
	historyList := newCompactList("Browse processed media", nil)
	loadingSpinner := spinner.New()
	loadingSpinner.Spinner = spinner.Dot
	return Model{
		Page: PageDiscover,
		Setup: JobSetupModel{
			ActionLabel:       "Start a new recovery",
			OutputPath:        "Not chosen yet",
			OutputDirectory:   ".",
			OutputFileName:    "",
			ActiveOutputField: OutputFieldFileName,
			DefaultPath:       "Not chosen yet",
			OutputFormat:      "ISO",
			FreeSpace:         "Unknown until an output location is selected",
			MethodLabel:       "Balanced recovery",
			CopyLabel:         "Not set (optional)",
		},
		Identity: ContentIdentityViewModel{
			Summary: "Finding usable optical drives.",
			Detail:  "Waiting for discovery results.",
		},
		PriorView: PriorProcessingViewModel{
			Kind:        PriorProcessingNone,
			HistoryLine: "History lookup is unavailable in this build.",
		},
		Recovery: RecoveryViewModel{
			Phase:      "Waiting to start",
			Status:     "No active job.",
			OutputPath: "Not chosen yet",
		},
		Details: DetailsViewModel{
			Lines: []string{
				"Drive: not selected",
				"Media: not identified",
				"Worker: idle",
			},
		},
		Notice: &NoticeModel{
			Text:     "Finding usable optical drives.",
			Severity: SeverityInfo,
		},
		NextRequestID:          2,
		ActiveDiscoveryRequest: 1,
		DirectoryInput:         directoryInput,
		FileNameInput:          fileNameInput,
		DetailsViewport:        detailsViewport,
		DriveList:              driveList,
		ActionList:             actionList,
		ResumeList:             resumeList,
		HistoryList:            historyList,
		LoadingSpinner:         loadingSpinner,
	}
}
