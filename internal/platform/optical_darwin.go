//go:build darwin

package platform

import (
	"fmt"
	"path/filepath"
	"strings"
)

func discoverHostOpticalDrives() ([]OpticalDrive, error) {
	nativeDrives, err := nativeDarwinDiscover()
	if err != nil {
		return nil, fmt.Errorf("discover macOS optical drives natively: %w", err)
	}
	drives := make([]OpticalDrive, 0, len(nativeDrives))
	for _, native := range nativeDrives {
		status := darwinDriveStatus(native)
		drives = append(drives, OpticalDrive{
			Path: native.Path, DisplayName: native.DisplayName, Status: status,
		})
	}
	return drives, nil
}

func darwinDriveStatus(native darwinNativeDrive) string {
	if native.Media && native.LogicalSectorSize > 0 && native.CapacityBytes > 0 {
		return "disc present"
	}
	switch native.State {
	case MediaProbeNoMedia:
		return "drive present; no media"
	case MediaProbeNotReady:
		return "drive present; media not ready"
	case MediaProbePermission:
		return "drive present; permission denied"
	case MediaProbeBusy:
		return "drive present; device busy"
	case MediaProbeFailure:
		return "drive present; media geometry unavailable"
	default:
		return "drive present; media state unavailable"
	}
}

func identifyHostOpticalMedia(path string) (OpticalMedia, error) {
	// Probe the selected node directly. A reinserted disc can recreate or
	// settle its /dev entry between drive refresh and the user's Enter press;
	// requiring a second directory scan creates a false "no longer available"
	// result during that window.
	native, err := inspectDarwinDisk(path)
	if err != nil {
		return OpticalMedia{}, err
	}
	if native.State == MediaProbeNoMedia || !native.Media {
		return OpticalMedia{}, &MediaInspectionError{Path: path, State: MediaProbeNoMedia}
	}
	if native.LogicalSectorSize == 0 || native.CapacityBytes == 0 {
		return OpticalMedia{}, &MediaInspectionError{Path: path, Operation: "read media geometry", State: MediaProbeFailure, Err: fmt.Errorf("zero media geometry")}
	}
	name := strings.TrimSpace(native.DisplayName)
	if name == "" {
		name = filepath.Base(path)
	}
	return OpticalMedia{
		Summary:             "Optical media detected.",
		Detail:              fmt.Sprintf("%s - %d-byte sectors - %d bytes", name, native.LogicalSectorSize, native.CapacityBytes),
		LogicalSectorSize:   native.LogicalSectorSize,
		CapacitySectors:     native.CapacityBytes / uint64(native.LogicalSectorSize),
		SuggestedOutputPath: filepath.Join(".", "discrescue-"+sanitizeDarwinName(name)+".iso"),
		Recoverable:         true,
	}, nil
}

func sanitizeDarwinName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-").Replace(name)
	if name == "" {
		return "optical-media"
	}
	return name
}
