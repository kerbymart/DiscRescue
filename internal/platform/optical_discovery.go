package platform

import (
	"errors"
	"fmt"
	"strings"

	"discrescue/internal/device"
)

type OSOpticalDiscovery struct {
	Process Process
}

func (d OSOpticalDiscovery) DiscoverOpticalDrives() ([]OpticalDrive, error) {
	if d.Process == nil {
		return nil, fmt.Errorf("discover optical drives: process runtime is not configured")
	}
	if text := strings.TrimSpace(d.Process.Getenv(discoveryErrorEnv)); text != "" {
		return nil, fmt.Errorf("discover optical drives: %s", text)
	}
	if strings.EqualFold(strings.TrimSpace(d.Process.Getenv(discoveryUnsupportedEnv)), "1") {
		return nil, fmt.Errorf("discover optical drives: %w", ErrUnsupportedEnvironment)
	}
	if text := strings.TrimSpace(d.Process.Getenv(discoveryDrivesEnv)); text != "" {
		return parseOpticalDrives(text), nil
	}
	return discoverHostOpticalDrives()
}

func (d OSOpticalDiscovery) IdentifyOpticalMedia(path string) (OpticalMedia, error) {
	media, probeErr := identifyHostOpticalMedia(path)
	if probeErr == nil {
		return ensureIdentityObservation(media), nil
	}
	var nativeErr *MediaInspectionError
	if errors.As(probeErr, &nativeErr) {
		return OpticalMedia{}, probeErr
	}
	drives, err := d.DiscoverOpticalDrives()
	if err != nil {
		return OpticalMedia{}, err
	}
	for _, drive := range drives {
		if drive.Path != path {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(drive.Status))
		switch {
		case strings.Contains(status, "empty"):
			return OpticalMedia{}, fmt.Errorf("inspect media: no disc detected in %s", drive.DisplayName)
		case strings.Contains(status, "permission"):
			return OpticalMedia{}, fmt.Errorf("inspect media: access to %s is restricted", drive.DisplayName)
		case strings.Contains(status, "unavailable"):
			return OpticalMedia{}, fmt.Errorf("inspect media: %s is no longer available", drive.DisplayName)
		}
		detail := strings.TrimSpace(drive.Path)
		if name := strings.TrimSpace(drive.DisplayName); name != "" {
			if detail != "" {
				detail = name + " - " + detail
			} else {
				detail = name
			}
		}
		return ensureIdentityObservation(OpticalMedia{Summary: "Media inspection completed.", Detail: detail, RecoverabilityNote: "The current build cannot start recovery from this media."}), nil
	}
	return OpticalMedia{}, fmt.Errorf("inspect media: drive %q is no longer available", path)
}

func (d OSOpticalDiscovery) OpticalCapabilities(path string) device.DriveCapabilities {
	return hostOpticalCapabilities(path)
}
