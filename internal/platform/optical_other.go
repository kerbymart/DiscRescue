//go:build !windows

package platform

import (
	"fmt"
	"path/filepath"
)

func discoverHostOpticalDrives() ([]OpticalDrive, error) {
	patterns := []string{"/dev/sr*", "/dev/cdrom*", "/dev/dvd*"}
	seen := map[string]struct{}{}
	drives := make([]OpticalDrive, 0, 4)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if _, exists := seen[match]; exists {
				continue
			}
			seen[match] = struct{}{}
			drives = append(drives, OpticalDrive{
				Path:        match,
				DisplayName: "Optical drive " + match,
				Status:      "available",
			})
		}
	}
	return drives, nil
}

func identifyHostOpticalMedia(path string) (OpticalMedia, error) {
	return OpticalMedia{}, fmt.Errorf("inspect media: mounted optical media inspection is not implemented for this platform")
}
