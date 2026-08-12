package platform

import (
	"discrescue/internal/catalog"
	"discrescue/internal/device"
)

type OpticalDrive struct {
	Path        string
	DisplayName string
	Status      string
}

type OpticalMedia struct {
	Summary             string
	Detail              string
	FileSystem          string
	VolumeLabel         string
	LogicalSectorSize   uint32
	CapacitySectors     uint64
	SuggestedOutputPath string
	Recoverable         bool
	RecoverabilityNote  string
	IdentityObservation *catalog.IdentityObservation
	PriorProcessing     *catalog.PriorProcessingResult
}

type OpticalDiscovery interface {
	DiscoverOpticalDrives() ([]OpticalDrive, error)
	IdentifyOpticalMedia(path string) (OpticalMedia, error)
}

// OpticalCapabilityProvider exposes operation-level support without leaking
// native handles into the application package.
type OpticalCapabilityProvider interface {
	OpticalCapabilities(path string) device.DriveCapabilities
}
