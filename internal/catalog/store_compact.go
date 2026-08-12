package catalog

import (
	"bytes"
	"sort"
)

func (s Store) Compact(bounds Bounds) (Store, error) {
	if err := s.Validate(); err != nil {
		return Store{}, err
	}
	if err := bounds.Validate(); err != nil {
		return Store{}, err
	}

	next := s
	next.Entries = append([]Entry(nil), s.Entries...)
	sort.Slice(next.Entries, func(i, j int) bool {
		if next.Entries[i].LastSeenUnixNano != next.Entries[j].LastSeenUnixNano {
			return next.Entries[i].LastSeenUnixNano > next.Entries[j].LastSeenUnixNano
		}
		return bytes.Compare(next.Entries[i].RecordID[:], next.Entries[j].RecordID[:]) < 0
	})
	if bounds.MaxEntries > 0 && len(next.Entries) > bounds.MaxEntries {
		next.Entries = next.Entries[:bounds.MaxEntries]
	}
	for i := range next.Entries {
		if bounds.MaxCapturesPerEntry > 0 && len(next.Entries[i].Captures) > bounds.MaxCapturesPerEntry {
			next.Entries[i].Captures = append([]CaptureIdentity(nil), next.Entries[i].Captures[len(next.Entries[i].Captures)-bounds.MaxCapturesPerEntry:]...)
		}
		if bounds.MaxJobRefsPerEntry > 0 && len(next.Entries[i].JobReferences) > bounds.MaxJobRefsPerEntry {
			next.Entries[i].JobReferences = append([]JobReference(nil), next.Entries[i].JobReferences[len(next.Entries[i].JobReferences)-bounds.MaxJobRefsPerEntry:]...)
		}
	}
	return next, nil
}
