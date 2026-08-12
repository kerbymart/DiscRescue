package app

import (
	tea "charm.land/bubbletea/v2"

	"discrescue/internal/device"
	"discrescue/internal/platform"
)

func (m ProgramModel) runOpticalEffect(request EffectRequestedMsg) tea.Msg {
	switch request.Kind {
	case EffectDiscoverDevices:
		drives, err := m.runtime.Optical.DiscoverOpticalDrives()
		return DevicesDiscoveredMsg{RequestID: request.RequestID, Generation: uint64(request.RequestID), Devices: toDeviceSummaries(drives), Err: err}
	case EffectIdentifyMedia:
		media, err := m.runtime.Optical.IdentifyOpticalMedia(request.DevicePath)
		if err != nil {
			return MediaIdentifiedMsg{RequestID: request.RequestID, Err: err}
		}
		return MediaIdentifiedMsg{
			RequestID: request.RequestID, Identity: identityViewModel(media, media.IdentityObservation),
			FileSystem: media.FileSystem, VolumeLabel: media.VolumeLabel, LogicalSectorSize: media.LogicalSectorSize,
			CapacitySectors: media.CapacitySectors, SuggestedOutputPath: media.SuggestedOutputPath,
			Recoverable: media.Recoverable, RecoverabilityNote: media.RecoverabilityNote,
			IdentityObservation: media.IdentityObservation, PriorProcessing: media.PriorProcessing,
		}
	case EffectEject:
		ejector, ok := m.runtime.Optical.(platform.OpticalEjector)
		if !ok {
			return EjectCompletedMsg{Request: request.Eject, Err: &device.OperationError{Code: device.ErrorUnsupported, Op: "eject optical drive", Device: device.DeviceRef{Path: request.DevicePath}, Detail: "the current platform adapter does not expose eject"}}
		}
		result, err := ejector.EjectOpticalDrive(request.DevicePath, request.Eject)
		return EjectCompletedMsg{Request: request.Eject, Result: result, Err: err}
	default:
		return unsupportedEffect(request)
	}
}
