package catalog

import "fmt"

type Store struct {
	LastSequence     uint64
	Entries          []Entry
	MutationLockHeld bool
}

func (s Store) Validate() error {
	for i, entry := range s.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("validate store entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (s Store) AcquireMutationLock() (Store, error) {
	if s.MutationLockHeld {
		return s, fmt.Errorf("acquire mutation lock: lock is already held")
	}
	next := s
	next.MutationLockHeld = true
	return next, nil
}

func (s Store) ReleaseMutationLock() Store {
	next := s
	next.MutationLockHeld = false
	return next
}

func (s Store) UpsertEntry(entry Entry) (Store, error) {
	if err := entry.Validate(); err != nil {
		return s, err
	}
	next := s
	replaced := false
	for i := range next.Entries {
		if next.Entries[i].RecordID == entry.RecordID {
			next.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		next.Entries = append(next.Entries, entry)
	}
	return next, nil
}

func validProcessingState(state ProcessingState) bool {
	switch state {
	case ProcessingObserved,
		ProcessingInProgress,
		ProcessingStoppedResumable,
		ProcessingCompleted,
		ProcessingCompletedVerified,
		ProcessingCompletedWithGaps,
		ProcessingFailed,
		ProcessingMerged:
		return true
	default:
		return false
	}
}
