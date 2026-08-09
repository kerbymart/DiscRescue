//go:build !linux && !darwin && !windows

package platform

import "discrescue/internal/device"

func hostOpticalCapabilities(path string) device.DriveCapabilities {
	unsupported := device.Capability{Status: device.SupportUnsupported, Detail: "native optical adapter unavailable"}
	return device.DriveCapabilities{MediaProbe: unsupported, RecoveryRead: unsupported, RawRead: unsupported, QueryReadSpeed: unsupported, SetReadSpeed: unsupported, NormalEject: unsupported, ForceEject: unsupported}
}
