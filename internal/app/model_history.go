package app

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
