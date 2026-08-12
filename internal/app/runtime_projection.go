package app

import (
	"fmt"
	"strings"

	"discrescue/internal/catalog"
	"discrescue/internal/platform"
)

func identityViewModel(media platform.OpticalMedia, observation *catalog.IdentityObservation) ContentIdentityViewModel {
	view := ContentIdentityViewModel{Summary: media.Summary, Detail: media.Detail}
	if observation == nil {
		return view
	}
	view.IdentityStatus = string(observation.Status)
	if observation.AttemptedSamples > 0 {
		view.Evidence = fmt.Sprintf("%d of %d fingerprint samples readable", observation.AvailableSamples, observation.AttemptedSamples)
	}
	return view
}
func toDeviceSummaries(drives []platform.OpticalDrive) []DeviceSummary {
	items := make([]DeviceSummary, 0, len(drives))
	seen := map[string]struct{}{}
	for _, drive := range drives {
		key := strings.TrimSpace(drive.Path)
		if key == "" {
			key = strings.TrimSpace(drive.DisplayName)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		displayName := strings.TrimSpace(drive.DisplayName)
		if displayName == "" {
			displayName = drive.Path
		}
		status := strings.TrimSpace(drive.Status)
		if status == "" {
			status = "available"
		}
		items = append(items, DeviceSummary{
			ID:          key,
			Path:        drive.Path,
			DisplayName: displayName,
			Status:      status,
		})
	}
	return items
}
