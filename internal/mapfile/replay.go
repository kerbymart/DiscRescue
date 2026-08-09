package mapfile

import "errors"

var ErrTruncatedRecord = errors.New("truncated final record")

func Replay(checkpoint Checkpoint, records []JournalRecord) Checkpoint {
	next := checkpoint
	for _, record := range records {
		next.LastSequence = record.Sequence
		if record.Extent == nil {
			continue
		}
		next.Extents = append(next.Extents, *record.Extent)
	}
	return next
}

func ReplayJournal(checkpoint Checkpoint, journal []byte) (Checkpoint, error) {
	next := checkpoint
	offset := 0
	expectedSequence := checkpoint.LastSequence + 1

	for offset < len(journal) {
		record, consumed, err := UnmarshalJournalRecord(journal[offset:])
		if err != nil {
			if errors.Is(err, ErrTruncatedRecord) {
				break
			}
			return Checkpoint{}, err
		}
		if record.Sequence != expectedSequence {
			return Checkpoint{}, errors.New("replay journal: non-monotonic sequence")
		}
		expectedSequence++
		offset += consumed
		next.LastSequence = record.Sequence
		if record.Type == RecordExtentStateChanged && record.Extent != nil {
			nextExtents, err := ApplyExtent(next.Extents, *record.Extent)
			if err != nil {
				return Checkpoint{}, err
			}
			next.Extents = nextExtents
		}
	}

	for index := range next.Extents {
		if next.Extents[index].State == SectorStateQueued {
			next.Extents[index].State = SectorStateUnknown
			next.Extents[index].Confidence = ConfidenceNone
		}
	}

	return next, nil
}
