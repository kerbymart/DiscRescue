package platform

import "strings"

const (
	discoveryDrivesEnv      = "DISKRESCUE_DISCOVERY_DRIVES"
	discoveryErrorEnv       = "DISKRESCUE_DISCOVERY_ERROR"
	discoveryUnsupportedEnv = "DISKRESCUE_DISCOVERY_UNSUPPORTED"
)

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
