package app

// reconcileDevices preserves a selection only when the same logical device
// remains present. A path change alone must not select a different drive.
func reconcileDevices(next []DeviceSummary, selected DeviceSummary) (devices []DeviceSummary, selection DeviceSummary, mediaInvalidated bool) {
	devices = append([]DeviceSummary(nil), next...)
	if selected.ID == "" && selected.Path != "" {
		selected.ID = selected.Path
	}
	for _, drive := range devices {
		id := drive.ID
		if id == "" {
			id = drive.Path
		}
		if id == selected.ID && id != "" {
			selection = drive
			if selected.MediaToken != "" && drive.MediaToken != "" && selected.MediaToken != drive.MediaToken {
				mediaInvalidated = true
			}
			return devices, selection, mediaInvalidated
		}
	}
	return devices, DeviceSummary{}, selected.ID != ""
}
