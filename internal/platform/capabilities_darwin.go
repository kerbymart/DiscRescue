//go:build darwin

package platform

import "discrescue/internal/device"

func hostOpticalCapabilities(path string) device.DriveCapabilities {
	return device.DriveCapabilities{
		MediaProbe:     device.Capability{Status: device.SupportSupported, Detail: "Darwin disk ioctl media probe"},
		RecoveryRead:   device.Capability{Status: device.SupportSupported, Detail: "read-only raw optical recovery"},
		RawRead:        device.Capability{Status: device.SupportSupported, Detail: "read-only raw device access"},
		QueryReadSpeed: device.Capability{Status: device.SupportUnknown, Detail: "drive-specific MMC query"},
		SetReadSpeed:   device.Capability{Status: device.SupportUnknown, Detail: "drive-specific MMC control"},
		NormalEject:    device.Capability{Status: device.SupportSupported, Detail: "Darwin DKIOCEJECT"},
		ForceEject:     device.Capability{Status: device.SupportSupported, Detail: "Darwin DKIOCEJECT after recovery handles close"},
	}
}
