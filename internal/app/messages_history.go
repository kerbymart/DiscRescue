package app

type PriorProcessingLookupMsg struct {
	RequestID int
	View      PriorProcessingViewModel
	Records   []PriorProcessingRecord
	Jobs      []ResumableJobViewModel
	Err       error
}

type ResumableJobsDiscoveredMsg struct {
	RequestID int
	Jobs      []ResumableJobViewModel
	Err       error
}

type ProcessedMediaDiscoveredMsg struct {
	RequestID int
	Items     []ProcessedMediaViewModel
	Err       error
}
