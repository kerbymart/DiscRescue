package app

import "discrescue/internal/catalog"

type DevicesDiscoveredMsg struct {
	RequestID  int
	Generation uint64
	Devices    []DeviceSummary
	Err        error
}

type MediaIdentifiedMsg struct {
	RequestID           int
	Identity            ContentIdentityViewModel
	FileSystem          string
	VolumeLabel         string
	LogicalSectorSize   uint32
	CapacitySectors     uint64
	SuggestedOutputPath string
	Recoverable         bool
	RecoverabilityNote  string
	IdentityObservation *catalog.IdentityObservation
	PriorProcessing     *catalog.PriorProcessingResult
	Err                 error
}
