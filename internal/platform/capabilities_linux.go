//go:build linux

package platform

import "discrescue/internal/device"

func hostOpticalCapabilities(path string) device.DriveCapabilities {
	return device.DriveCapabilities{
		MediaProbe:     device.Capability{Status: device.SupportSupported, Detail: "Linux optical device probe"},
		RecoveryRead:   device.Capability{Status: device.SupportSupported, Detail: "Linux raw optical recovery"},
		RawRead:        device.Capability{Status: device.SupportSupported, Detail: "Linux read-only device access"},
		QueryReadSpeed: device.Capability{Status: device.SupportSupported, Detail: "MMC query path available"},
		SetReadSpeed:   device.Capability{Status: device.SupportSupported, Detail: "MMC SET CD SPEED path available"},
		NormalEject:    device.Capability{Status: device.SupportSupported, Detail: "Linux optical drive ioctl eject"},
		ForceEject:     device.Capability{Status: device.SupportSupported, Detail: "Linux explicit eject escalation"},
	}
}
