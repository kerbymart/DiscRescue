package platform

import (
	"errors"
	"fmt"
	"strings"
)

const (
	discoveryDrivesEnv      = "DISKRESCUE_DISCOVERY_DRIVES"
	discoveryErrorEnv       = "DISKRESCUE_DISCOVERY_ERROR"
	discoveryUnsupportedEnv = "DISKRESCUE_DISCOVERY_UNSUPPORTED"
)

var ErrUnsupportedEnvironment = errors.New("unsupported environment")

type OpticalDrive struct {
	Path        string
	DisplayName string
	Status      string
}

type OpticalMedia struct {
	Summary string
	Detail  string
}

type OpticalDiscovery interface {
	DiscoverOpticalDrives() ([]OpticalDrive, error)
	IdentifyOpticalMedia(path string) (OpticalMedia, error)
}

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
		return OpticalMedia{
			Summary: "Media inspection completed.",
			Detail:  detail,
		}, nil
	}
	return OpticalMedia{}, fmt.Errorf("inspect media: drive %q is no longer available", path)
}

func parseOpticalDrives(text string) []OpticalDrive {
	parts := strings.Split(text, ";")
	items := make([]OpticalDrive, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, "|")
		drive := OpticalDrive{}
		if len(fields) > 0 {
			drive.Path = strings.TrimSpace(fields[0])
		}
		if len(fields) > 1 {
			drive.DisplayName = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			drive.Status = strings.TrimSpace(fields[2])
		}
		if drive.Path == "" && drive.DisplayName == "" {
			continue
		}
		key := drive.Path
		if key == "" {
			key = drive.DisplayName
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, drive)
	}
	return items
}
