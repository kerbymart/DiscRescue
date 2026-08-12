package app

import (
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"discrescue/internal/device"
	"discrescue/internal/platform"
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
	PageEjectConfirm
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
	PendingEject            device.EjectRequest
	Notice                  *NoticeModel
	LastError               error
	Quitting                bool
	Monochrome              bool
	DarkBackground          bool
	NextRequestID           int
	ActiveDiscoveryRequest  int
	// PreserveEjectedDrive keeps the physical drive selectable when a tray
	// opening removes its /dev node or media record during refresh.
	PreserveEjectedDrive  bool
	EjectedDrive          DeviceSummary
	ActiveMediaRequest    int
	ActiveLookupRequest   int
	ActiveTargetRequest   int
	ActiveResumeRequest   int
	ActiveHistoryRequest  int
	DirectoryInput        textinput.Model
	FileNameInput         textinput.Model
	DetailsViewport       viewport.Model
	DriveList             list.Model
	ActionList            list.Model
	ResumeList            list.Model
	HistoryList           list.Model
	LoadingSpinner        spinner.Model
	RestartLoadingSpinner bool
}

func NewModel() Model {
	directoryInput := textinput.New()
	directoryInput.Prompt = ""
	directoryInput.CharLimit = 4096
	directoryInput.SetValue(".")
	stylePathInput(&directoryInput)
	fileNameInput := textinput.New()
	fileNameInput.Prompt = ""
	fileNameInput.CharLimit = 255
	stylePathInput(&fileNameInput)
	detailsViewport := viewport.New()
	driveList := newCompactList("Choose a drive", nil, true)
	actionList := newCompactList("What do you want to do?", recoveryActionItems(), true)
	resumeList := newCompactList("Resume unfinished recovery", nil, true)
	historyList := newCompactList("Browse processed media", nil, true)
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
			Method:            platform.RecoveryMethodBalanced,
			ReadSpeed:         device.DefaultReadSpeedRequest(),
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
		DarkBackground:         true,
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
