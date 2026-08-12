package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"time"
)

func ReplayJournal(snapshot Snapshot, journal []byte) (Store, error) {
	if err := snapshot.Validate(); err != nil {
		return Store{}, err
	}
	store := Store{LastSequence: snapshot.LastSequence, Entries: append([]Entry(nil), snapshot.Entries...)}
	offset := 0
	expectedSequence := snapshot.LastSequence + 1
	for offset < len(journal) {
		record, consumed, err := UnmarshalJournalRecord(journal[offset:])
		if err != nil {
			if errors.Is(err, ErrTruncatedCatalogRecord) {
				break
			}
			return Store{}, err
		}
		if record.Sequence < expectedSequence {
			offset += consumed
			continue
		}
		if record.Sequence != expectedSequence {
			return Store{}, fmt.Errorf("replay journal: non-monotonic sequence")
		}
		expectedSequence++
		event, err := DecodeCatalogEvent(record)
		if err != nil {
			return Store{}, err
		}
		store, err = ApplyEvent(store, event, record.Sequence)
		if err != nil {
			return Store{}, err
		}
		offset += consumed
	}
	return store, store.Validate()
}

func ApplyEvent(store Store, event CatalogEvent, sequence uint64) (Store, error) {
	if err := event.Validate(); err != nil {
		return store, err
	}
	next, err := store.UpsertEntry(event.Entry)
	if err != nil {
		return store, err
	}
	next.LastSequence = sequence
	return next, nil
}

func BuildSnapshotRecord(store Store, bounds Bounds) (Snapshot, error) {
	compacted, err := store.Compact(bounds)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{LastSequence: compacted.LastSequence, Entries: append([]Entry(nil), compacted.Entries...)}, nil
}

func TruncateJournalAfterSnapshot(journal []byte, snapshot Snapshot) []byte {
	offset := 0
	for offset < len(journal) {
		record, consumed, err := UnmarshalJournalRecord(journal[offset:])
		if err != nil {
			if errors.Is(err, ErrTruncatedCatalogRecord) {
				return nil
			}
			return append([]byte(nil), journal[offset:]...)
		}
		if record.Sequence > snapshot.LastSequence {
			return append([]byte(nil), journal[offset:]...)
		}
		offset += consumed
	}
	return nil
}

func unixNanoUTC(value int64) time.Time {
	return time.Unix(0, value).UTC()
}

func BuildTempSnapshotName(base string) string {
	return base + ".tmp"
}

func ReplaceSnapshotAtomically(current, replacement []byte) []byte {
	return bytes.Clone(replacement)
}
