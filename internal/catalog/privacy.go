package catalog

import "fmt"

type RecordingPreference struct {
	AutomaticRecordingDisabled bool
}

func (p RecordingPreference) AutomaticRecordingEnabled() bool {
	return !p.AutomaticRecordingDisabled
}

func (p RecordingPreference) ShouldRecordAutomatically() bool {
	return !p.AutomaticRecordingDisabled
}

func (p RecordingPreference) ShouldRecordManualAction() bool {
	return true
}

func (s Store) HideRecord(recordID RecordID) (Store, error) {
	next := s
	for i := range next.Entries {
		if next.Entries[i].RecordID != recordID {
			continue
		}
		next.Entries[i].Hidden = true
		return next, nil
	}
	return s, fmt.Errorf("hide record: record %x not found", recordID)
}

func (s Store) ForgetMissingPathReferences(recordID RecordID) (Store, error) {
	next := s
	for i := range next.Entries {
		if next.Entries[i].RecordID != recordID {
			continue
		}
		entry := next.Entries[i]
		filtered := entry.JobReferences[:0]
		for _, reference := range entry.JobReferences {
			if reference.FilesPresent {
				filtered = append(filtered, reference)
			}
		}
		entry.JobReferences = append([]JobReference(nil), filtered...)
		if entry.PreferredJobID != (RecordID{}) && !containsJobReference(entry.JobReferences, entry.PreferredJobID) {
			entry.PreferredJobID = RecordID{}
		}
		next.Entries[i] = entry
		return next, nil
	}
	return s, fmt.Errorf("forget missing path references: record %x not found", recordID)
}

func containsJobReference(references []JobReference, jobID RecordID) bool {
	for _, reference := range references {
		if reference.JobID == jobID {
			return true
		}
	}
	return false
}
