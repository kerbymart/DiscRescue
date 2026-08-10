package platform

import (
	"errors"
	"fmt"
	"strings"

	"discrescue/internal/catalog"
	"discrescue/internal/device"
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
	Summary             string
	Detail              string
	FileSystem          string
	VolumeLabel         string
	LogicalSectorSize   uint32
	CapacitySectors     uint64
	SuggestedOutputPath string
	Recoverable         bool
	RecoverabilityNote  string
	IdentityObservation *catalog.IdentityObservation
	PriorProcessing     *catalog.PriorProcessingResult
}

// MediaProbeState describes why a selected optical device could not be
// inspected. It keeps media absence distinct from device removal and from
// host I/O failures so callers can present a truthful next action.
type MediaProbeState string

const (
	MediaProbeNoMedia     MediaProbeState = "no_media"
	MediaProbeNotReady    MediaProbeState = "not_ready"
	MediaProbePermission  MediaProbeState = "permission_denied"
	MediaProbeBusy        MediaProbeState = "busy"
	MediaProbeUnavailable MediaProbeState = "unavailable"
	MediaProbeFailure     MediaProbeState = "failure"
)

// MediaInspectionError preserves the native operation and underlying error
// returned while probing a selected optical device.
type MediaInspectionError struct {
	Path      string
	Operation string
	State     MediaProbeState
	Err       error
}

func (e *MediaInspectionError) Error() string {
	if e == nil {
		return "inspect media: unknown probe failure"
	}
	switch e.State {
	case MediaProbeNoMedia:
		return fmt.Sprintf("inspect media: no disc is inserted in %s", e.Path)
	case MediaProbeNotReady:
		return fmt.Sprintf("inspect media: media in %s is not ready", e.Path)
	case MediaProbeUnavailable:
		return fmt.Sprintf("inspect media: drive %q is no longer available", e.Path)
	}
	if e.Operation == "" {
		return fmt.Sprintf("inspect media: %s", e.Err)
	}
	if e.Err == nil {
		return fmt.Sprintf("inspect media: %s %s failed", e.Operation, e.Path)
	}
	return fmt.Sprintf("inspect media: %s %s: %v", e.Operation, e.Path, e.Err)
}

func (e *MediaInspectionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type OpticalDiscovery interface {
	DiscoverOpticalDrives() ([]OpticalDrive, error)
	IdentifyOpticalMedia(path string) (OpticalMedia, error)
}

// OpticalCapabilityProvider exposes operation-level support without leaking
// native handles into the application package.
type OpticalCapabilityProvider interface {
	OpticalCapabilities(path string) device.DriveCapabilities
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
		return ensureIdentityObservation(OpticalMedia{
			Summary:            "Media inspection completed.",
			Detail:             detail,
			RecoverabilityNote: "The current build cannot start recovery from this media.",
		}), nil
	}
	return OpticalMedia{}, fmt.Errorf("inspect media: drive %q is no longer available", path)
}

func (d OSOpticalDiscovery) OpticalCapabilities(path string) device.DriveCapabilities {
	return hostOpticalCapabilities(path)
}

func ensureIdentityObservation(media OpticalMedia) OpticalMedia {
	if media.IdentityObservation == nil {
		media.IdentityObservation = &catalog.IdentityObservation{
			Status: catalog.IdentityUnavailable,
			Detail: "Content fingerprint collection is unavailable for this media inspection.",
		}
	}
	return media
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
