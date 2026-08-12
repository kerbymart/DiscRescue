//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const ioctlDiskGetDriveGeometryEx = 0x000700a0

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
		label, fsName, totalBytes, err := windowsVolumeInfo(root)
		status := "no media"
		if rawInfo, rawErr := queryWindowsRawOpticalMedia(root); rawErr == nil {
			status = "disc present"
			if err == nil && fsName != "" && totalBytes > 0 {
				status = fsName
				if label != "" {
					status = fsName + " - " + label
				}
			} else if rawInfo.CapacitySectors > 0 {
				status = fmt.Sprintf("disc present - %d sectors", rawInfo.CapacitySectors)
			}
		}
		drives = append(drives, OpticalDrive{
			Path:        root,
			DisplayName: fmt.Sprintf("Optical drive %c:", letter),
			Status:      status,
		})
	}
	return drives, nil
}

func windowsVolumeInfo(root string) (label string, fileSystem string, totalBytes uint64, err error) {
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "", "", 0, err
	}
	var labelBuf [256]uint16
	var fsBuf [256]uint16
	var freeCaller uint64
	var freeTotal uint64
	if err := windows.GetVolumeInformation(rootPtr, &labelBuf[0], uint32(len(labelBuf)), nil, nil, nil, &fsBuf[0], uint32(len(fsBuf))); err != nil {
		return "", "", 0, err
	}
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &freeCaller, &totalBytes, &freeTotal); err != nil {
		return "", "", 0, err
	}
	return windows.UTF16ToString(labelBuf[:]), windows.UTF16ToString(fsBuf[:]), totalBytes, nil
}
