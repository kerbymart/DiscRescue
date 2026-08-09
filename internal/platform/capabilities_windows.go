//go:build windows

package platform

import "discrescue/internal/device"

func hostOpticalCapabilities(path string) device.DriveCapabilities {
	return device.DriveCapabilities{
		MediaProbe:     device.Capability{Status: device.SupportSupported, Detail: "Windows optical drive probe"},
		RecoveryRead:   device.Capability{Status: device.SupportSupported, Detail: "Windows raw optical recovery"},
		RawRead:        device.Capability{Status: device.SupportSupported, Detail: "Windows read-only device access"},
		QueryReadSpeed: device.Capability{Status: device.SupportUnknown, Detail: "drive-specific MMC query"},
		SetReadSpeed:   device.Capability{Status: device.SupportUnknown, Detail: "drive-specific MMC control"},
		NormalEject:    device.Capability{Status: device.SupportSupported, Detail: "Windows storage eject"},
		ForceEject:     device.Capability{Status: device.SupportSupported, Detail: "Windows explicit eject escalation"},
	}
}
