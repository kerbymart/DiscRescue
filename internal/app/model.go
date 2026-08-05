package app

type Page uint8

const (
	PageDiscover Page = iota
	PageChooseDrive
	PagePriorProcessing
	PageChooseAction
	PageChooseOutput
	PageReview
	PageRecovering
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

type JobSetupModel struct {
	ActionLabel  string
	OutputPath   string
	OutputFormat string
	FreeSpace    string
}

type ContentIdentityViewModel struct {
	Summary string
	Detail  string
}

type PriorProcessingRecord struct {
	Title  string
	Detail string
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
	UnreadableSectors string
}

type RecoveryViewModel struct {
	Phase             string
	RecoveredSectors  uint64
	TotalSectors      uint64
	UnreadableSectors uint64
	Status            string
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
	RecoveredSectors  uint64
	TotalSectors      uint64
	UnreadableSectors uint64
	Status            string
}

type JobSummary struct {
	Outcome           string
	ImagePath         string
	MapPath           string
	NextAction        string
	UnresolvedSectors uint64
}

type Model struct {
	Page         Page
	PreviousPage Page
	Width        int
	Height       int
	Devices      []DeviceSummary
	Cursor       int
	Setup        JobSetupModel
	Identity     ContentIdentityViewModel
	PriorView    PriorProcessingViewModel
	PriorRecords []PriorProcessingRecord
	Recovery     RecoveryViewModel
	Details      DetailsViewModel
	Dialog       *DialogModel
	Notice       *NoticeModel
	LastError    error
	Quitting     bool
	Monochrome   bool
}

func NewModel() Model {
	return Model{
		Page: PageDiscover,
		Setup: JobSetupModel{
			ActionLabel:  "Start a new recovery",
			OutputPath:   "D:/Archives/discrescue-image.iso",
			OutputFormat: "ISO",
			FreeSpace:    "Checking free space...",
		},
		Identity: ContentIdentityViewModel{
			Summary: "Finding usable drives and resumable jobs.",
			Detail:  "The recovery shell is ready for simulator-driven workflows.",
		},
		PriorView: PriorProcessingViewModel{
			Kind:        PriorProcessingNone,
			HistoryLine: "History: no matching contents found on this computer",
		},
		Recovery: RecoveryViewModel{
			Phase:  "Waiting to start",
			Status: "No active job.",
		},
		Details: DetailsViewModel{
			Lines: []string{
				"Drive: not selected",
				"Media: not identified",
				"Worker: idle",
			},
		},
		Notice: &NoticeModel{
			Text:     "Finding usable drives and resumable jobs.",
			Severity: SeverityInfo,
		},
	}
}
