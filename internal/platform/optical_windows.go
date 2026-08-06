//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func discoverHostOpticalDrives() ([]OpticalDrive, error) {
	drives := make([]OpticalDrive, 0, 4)
	for letter := 'A'; letter <= 'Z'; letter++ {
		root := fmt.Sprintf("%c:\\", letter)
		ptr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			return nil, fmt.Errorf("discover optical drives: encode root %q: %w", root, err)
		}
		if windows.GetDriveType(ptr) != windows.DRIVE_CDROM {
			continue
		}
		drives = append(drives, OpticalDrive{
			Path:        root,
			DisplayName: fmt.Sprintf("Optical drive %c:", letter),
			Status:      "available",
		})
	}
	return drives, nil
}
