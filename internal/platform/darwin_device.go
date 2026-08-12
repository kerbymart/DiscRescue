//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func nativeDarwinDiscover() ([]darwinNativeDrive, error) {
	paths, err := darwinDeviceCandidates()
	if err != nil {
		return nil, err
	}
	configured := darwinOpticalDevicesConfigured()
	var drives []darwinNativeDrive
	for _, path := range paths {
		drive, err := inspectDarwinDisk(path)
		if shouldRetainDarwinCandidate(configured, err) {
			drives = append(drives, drive)
			continue
		}
	}
	return drives, nil
}

func shouldRetainDarwinCandidate(configured bool, err error) bool {
	if err == nil {

		return true
	}
	if !configured {

		return false
	}
	var probeErr *MediaInspectionError
	return errors.As(err, &probeErr) && probeErr.State != MediaProbeUnavailable
}

func darwinOpticalDevicesConfigured() bool {
	return strings.TrimSpace(os.Getenv("DISKRESCUE_DARWIN_OPTICAL_DEVICES")) != ""
}

func darwinDeviceCandidates() ([]string, error) {

	if configured := strings.TrimSpace(os.Getenv("DISKRESCUE_DARWIN_OPTICAL_DEVICES")); configured != "" {
		var paths []string
		for _, value := range strings.Split(configured, ",") {
			path := strings.TrimSpace(value)
			if path != "" {
				paths = append(paths, path)
			}
		}
		return paths, nil
	}
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("enumerate Darwin device nodes: %w", err)
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "disk") && isDecimal(name[len("disk"):]) {
			paths = append(paths, filepath.Join("/dev", name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}
