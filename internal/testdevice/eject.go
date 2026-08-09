package testdevice

import (
	"fmt"

	"discrescue/internal/device"
)

// EjectSpec gives simulator scenarios deterministic eject outcomes without
// opening a host device.
type EjectSpec struct {
	NormalSupported bool
	ForceSupported  bool
	NormalFails     bool
	ForceFails      bool
	MediaPresent    bool
}

func (s EjectSpec) Apply(request device.EjectRequest, owner device.DriveOwner) (device.EjectResult, error) {
	if err := request.Validate(); err != nil {
		return device.EjectResult{Requested: request}, err
	}
	if request.Mode == device.EjectNormal && owner != device.DriveUnowned {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorBusy, Op: "simulator normal eject", Detail: "drive is owned"}
	}
	supported := s.NormalSupported
	fails := s.NormalFails
	if request.Mode == device.EjectForce {
		supported = s.ForceSupported
		fails = s.ForceFails
	}
	if !supported {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorUnsupported, Op: "simulator eject", Detail: fmt.Sprintf("%s eject unsupported", request.Mode)}
	}
	if fails {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorDeviceFailure, Op: "simulator eject", Detail: fmt.Sprintf("%s eject failed", request.Mode)}
	}
	s.MediaPresent = false
	return device.EjectResult{Requested: request, Status: device.OperationCompleted, Verification: device.EjectConfirmed, Detail: "simulator confirmed media removal"}, nil
}
