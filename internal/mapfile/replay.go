package mapfile

func Replay(checkpoint Checkpoint, records []JournalRecord) Checkpoint {
	next := checkpoint
	for _, record := range records {
		next.LastSequence = record.Sequence
		next.Extents = append(next.Extents, record.Extent)
	}
	return next
}
