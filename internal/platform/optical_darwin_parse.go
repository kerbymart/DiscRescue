package platform

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func normalizeDarwinOpticalDevice(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/dev/") {
		return "", fmt.Errorf("device path %q is not a Darwin disk device", path)
	}
	name := strings.TrimPrefix(path, "/dev/")
	if strings.HasPrefix(name, "r") {
		name = strings.TrimPrefix(name, "r")
	}
	if !strings.HasPrefix(name, "disk") || len(name) == len("disk") {
		return "", fmt.Errorf("device path %q is not a Darwin disk device", path)
	}
	for _, r := range strings.TrimPrefix(name, "disk") {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("device path %q is not a whole Darwin disk", path)
		}
	}
	return filepath.Join("/dev", "r"+name), nil
}

func parseDarwinDiskutilList(text string) []OpticalDrive {
	var drives []OpticalDrive
	seen := map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "/dev/disk") || !strings.HasSuffix(line, ":") {
			continue
		}
		path := strings.TrimSuffix(strings.Fields(line)[0], ":")
		if _, err := normalizeDarwinOpticalDevice(path); err != nil {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		drives = append(drives, OpticalDrive{Path: path, DisplayName: "Optical drive " + path, Status: "disc present"})
	}
	return drives
}

type darwinDiskInfo struct {
	DevicePath        string
	MediaName         string
	OpticalMedia      bool
	FileSystem        string
	VolumeName        string
	LogicalSectorSize uint32
	CapacityBytes     uint64
}

func parseDarwinDiskutilInfo(text string) (darwinDiskInfo, error) {
	var info darwinDiskInfo
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "Device Node":
			info.DevicePath = value
		case "Media Name", "Device / Media Name":
			info.MediaName = value
		case "Optical Media":
			info.OpticalMedia = strings.EqualFold(value, "Yes")
		case "Optical Media Type":
			info.OpticalMedia = value != "" && !strings.EqualFold(value, "None")
		case "Optical Drive Type":
			info.OpticalMedia = value != "" && !strings.EqualFold(value, "None")
		case "File System Personality":
			info.FileSystem = value
		case "Volume Name":
			info.VolumeName = value
		case "Device Block Size":
			if sectorSize, ok := firstDecimal(value); ok {
				info.LogicalSectorSize = uint32(sectorSize)
			}
		case "Disk Size":
			if open := strings.IndexByte(value, '('); open >= 0 {
				value = value[open+1:]
			}
			if capacity, ok := firstDecimal(value); ok {
				info.CapacityBytes = capacity
			}
		}
	}
	if info.DevicePath == "" {
		return darwinDiskInfo{}, fmt.Errorf("diskutil did not report a device node")
	}
	return info, nil
}

type darwinDrutilDrive struct {
	Index        int
	Vendor       string
	Product      string
	Revision     string
	Bus          string
	SupportLevel string
}

func parseDarwinDrutilList(text string) []darwinDrutilDrive {
	var drives []darwinDrutilDrive
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		index, err := strconv.Atoi(fields[0])
		if err != nil || index < 1 {
			continue
		}
		drives = append(drives, darwinDrutilDrive{
			Index: index, Vendor: fields[1], Product: strings.Join(fields[2:len(fields)-3], " "),
			Revision: fields[len(fields)-3], Bus: fields[len(fields)-2], SupportLevel: fields[len(fields)-1],
		})
	}
	return drives
}

func drutilDrivePath(index int) string { return fmt.Sprintf("drutil:%d", index) }

func parseDrutilDrivePath(path string) (int, bool) {
	if !strings.HasPrefix(path, "drutil:") {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimPrefix(path, "drutil:"))
	return index, err == nil && index > 0
}

func darwinDriveDisplayName(info darwinDiskInfo, path string) string {
	label := strings.TrimSpace(info.VolumeName)
	if label == "" {
		label = strings.TrimSpace(info.MediaName)
	}
	if label == "" {
		label = "Optical media"
	}
	return fmt.Sprintf("%s (%s)", label, path)
}

func firstDecimal(text string) (uint64, bool) {
	for _, field := range strings.FieldsFunc(text, func(r rune) bool { return r < '0' || r > '9' }) {
		if field == "" {
			continue
		}
		value, err := strconv.ParseUint(field, 10, 64)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}
