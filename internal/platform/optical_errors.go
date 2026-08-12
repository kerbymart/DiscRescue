package platform

import (
	"errors"
	"fmt"
)

var ErrUnsupportedEnvironment = errors.New("unsupported environment")

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
