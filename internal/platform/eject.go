package platform

import "discrescue/internal/device"

// OpticalEjector is optional so discovery-only test doubles and platforms
// without an eject implementation can report unsupported capability explicitly.
type OpticalEjector interface {
	EjectOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error)
	EjectCapability(path string) device.Capability
}

func (d OSOpticalDiscovery) EjectCapability(path string) device.Capability {
	return ejectHostCapability(path)
}

func (d OSOpticalDiscovery) EjectOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	if err := request.Validate(); err != nil {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorInvalidRequest, Op: "eject", Device: device.DeviceRef{Path: path}, Cause: err}
	}
	return ejectHostOpticalDrive(path, request)
}
