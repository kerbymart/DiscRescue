//go:build windows

package platform

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const ioctlDiskGetDriveGeometryEx = 0x000700a0

type diskGeometry struct {
	Cylinders         int64
	MediaType         uint32
	TracksPerCylinder uint32
	SectorsPerTrack   uint32
	BytesPerSector    uint32
}

type diskGeometryExHeader struct {
	Geometry diskGeometry
	DiskSize int64
}

type rawOpticalMediaInfo struct {
	LogicalSectorSize uint32
	CapacitySectors   uint64
}

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

func identifyWindowsOpticalMedia(path string) (OpticalMedia, error) {
	root := path
	if !strings.HasSuffix(root, "\\") {
		root += "\\"
	}

	rawInfo, err := queryWindowsRawOpticalMedia(root)
	if err != nil {
		return OpticalMedia{}, windowsMediaProbeError(root, err)
	}

	label, fsName, totalBytes, err := windowsVolumeInfo(root)
	if err != nil || fsName == "" || totalBytes == 0 {
		totalBytes = rawInfo.CapacitySectors * uint64(rawInfo.LogicalSectorSize)
		fsName = ""
		label = ""
	}

	name := strings.TrimSpace(label)
	if name == "" {
		name = strings.TrimSuffix(root, "\\")
	}
	detailParts := []string{
		fmt.Sprintf("%d-byte sectors", rawInfo.LogicalSectorSize),
		fmt.Sprintf("%d sectors", rawInfo.CapacitySectors),
		fmt.Sprintf("%d bytes", totalBytes),
	}
	if fsName != "" {
		detailParts = append([]string{fsName}, detailParts...)
	}
	if label != "" {
		detailParts = append(detailParts, label)
	}
	return OpticalMedia{
		Summary:             "Optical media detected.",
		Detail:              strings.Join(detailParts, " - "),
		FileSystem:          fsName,
		VolumeLabel:         label,
		LogicalSectorSize:   rawInfo.LogicalSectorSize,
		CapacitySectors:     rawInfo.CapacitySectors,
		SuggestedOutputPath: filepath.Join(".", sanitizeOutputName(fmt.Sprintf("discrescue-%s-%s.iso", strings.TrimSuffix(root, "\\"), name))),
		Recoverable:         true,
		RecoverabilityNote:  "",
	}, nil
}

func windowsMediaProbeError(path string, err error) error {
	state := MediaProbeFailure
	switch {
	case errors.Is(err, windows.ERROR_NO_MEDIA_IN_DRIVE):
		state = MediaProbeNoMedia
	case errors.Is(err, windows.ERROR_NOT_READY):
		state = MediaProbeNotReady
	case errors.Is(err, windows.ERROR_ACCESS_DENIED):
		state = MediaProbePermission
	case errors.Is(err, windows.ERROR_BUSY), errors.Is(err, windows.ERROR_BUSY_DRIVE):
		state = MediaProbeBusy
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND), errors.Is(err, windows.ERROR_PATH_NOT_FOUND), errors.Is(err, windows.ERROR_DEVICE_NOT_CONNECTED):
		state = MediaProbeUnavailable
	}
	return &MediaInspectionError{Path: path, Operation: "open or inspect", State: state, Err: err}
}

func queryWindowsRawOpticalMedia(root string) (rawOpticalMediaInfo, error) {
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(rawVolumePath(root)),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return rawOpticalMediaInfo{}, err
	}
	defer windows.CloseHandle(handle)

	var returned uint32
	buffer := make([]byte, 256)
	if err := windows.DeviceIoControl(
		handle,
		ioctlDiskGetDriveGeometryEx,
		nil,
		0,
		&buffer[0],
		uint32(len(buffer)),
		&returned,
		nil,
	); err != nil {
		return rawOpticalMediaInfo{}, err
	}
	if returned < uint32(unsafe.Sizeof(diskGeometryExHeader{})) {
		return rawOpticalMediaInfo{}, fmt.Errorf("raw geometry response too short: %d bytes", returned)
	}
	header := (*diskGeometryExHeader)(unsafe.Pointer(&buffer[0]))
	if header.Geometry.BytesPerSector == 0 {
		return rawOpticalMediaInfo{}, fmt.Errorf("raw geometry reported zero-byte sectors")
	}
	if header.DiskSize <= 0 {
		return rawOpticalMediaInfo{}, fmt.Errorf("raw geometry reported zero capacity")
	}
	capacitySectors := uint64(header.DiskSize) / uint64(header.Geometry.BytesPerSector)
	if capacitySectors == 0 {
		return rawOpticalMediaInfo{}, fmt.Errorf("raw geometry reported zero logical sectors")
	}
	return rawOpticalMediaInfo{
		LogicalSectorSize: header.Geometry.BytesPerSector,
		CapacitySectors:   capacitySectors,
	}, nil
}

func sanitizeOutputName(name string) string {
	replacer := strings.NewReplacer(":", "", "\\", "-", "/", "-", "*", "-", "?", "", "\"", "", "<", "", ">", "", "|", "", " ", "-")
	return replacer.Replace(name)
}

func rawVolumePath(root string) string {
	root = strings.TrimSpace(root)
	root = strings.TrimSuffix(root, "\\")
	return `\\.\` + root
}
