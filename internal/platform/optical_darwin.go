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
		status := "drive present; media state unavailable"
		if native.Media && native.LogicalSectorSize > 0 && native.CapacityBytes > 0 {
			status = "disc present"
		}
		drives = append(drives, OpticalDrive{
			Path: native.Path, DisplayName: native.DisplayName, Status: status,
		})
	}
	return drives, nil
}

func identifyHostOpticalMedia(path string) (OpticalMedia, error) {
	// Probe the selected node directly. A reinserted disc can recreate or
	// settle its /dev entry between drive refresh and the user's Enter press;
	// requiring a second directory scan creates a false "no longer available"
	// result during that window.
	native, ok := inspectDarwinDisk(path)
	if !ok {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media: drive %q is no longer available or has no readable geometry", path)
	}
	if !native.Media || native.LogicalSectorSize == 0 || native.CapacityBytes == 0 {
		return OpticalMedia{}, fmt.Errorf("inspect macOS optical media: %s is present but has no readable media geometry", native.DisplayName)
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
