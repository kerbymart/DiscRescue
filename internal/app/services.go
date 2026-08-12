package app

import (
	"discrescue/internal/device"
	"discrescue/internal/platform"
)

// OpticalService is the optical capability required by the application
// workflow. Eject remains an optional capability so discovery-only providers
// do not implement unrelated control operations.
type OpticalService interface {
	DiscoverOpticalDrives() ([]platform.OpticalDrive, error)
	IdentifyOpticalMedia(path string) (platform.OpticalMedia, error)
}

type OpticalEjector interface {
	EjectOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error)
	EjectCapability(path string) device.Capability
}

type RecoveryService interface {
	StartImageRecovery(input platform.RecoveryInput) (platform.RecoveryJob, error)
	InspectRecoveryTarget(input platform.RecoveryInput) (platform.RecoveryTargetStatus, error)
}

// Services is the application-owned dependency boundary. Platform.Runtime is
// assembled by the composition root and is not retained by the TUI model.
type Services struct {
	Optical  OpticalService
	Recovery RecoveryService
}
