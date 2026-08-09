package mapfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

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

// ReplayJournalWithinCapacity replays a journal and rejects any resulting
// extent outside the selected media geometry.
func ReplayJournalWithinCapacity(checkpoint Checkpoint, journal []byte, expectedSectorCount uint64) (Checkpoint, error) {
	replayed, err := ReplayJournal(checkpoint, journal)
	if err != nil {
		return Checkpoint{}, err
	}
	if err := ValidateExtentSetWithinCapacity(replayed.Extents, expectedSectorCount); err != nil {
		return Checkpoint{}, fmt.Errorf("replay journal capacity: %w", err)
	}
	return replayed, nil
}

// ReplayJournalReader replays journal records from a stream using a bounded
// record buffer. An incomplete final record at EOF is ignored.
func ReplayJournalReader(checkpoint Checkpoint, reader io.Reader) (Checkpoint, error) {
	if reader == nil {
		return Checkpoint{}, errors.New("replay journal reader: reader is required")
	}
	next := checkpoint
	expectedSequence := checkpoint.LastSequence + 1
	header := make([]byte, 14)
	for {
		read, err := io.ReadFull(reader, header)
		if err != nil {
			if (errors.Is(err, io.EOF) && read == 0) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return Checkpoint{}, fmt.Errorf("replay journal reader: read record header: %w", err)
		}
		payloadLength := binary.LittleEndian.Uint32(header[10:14])
		if payloadLength > MaxJournalPayloadBytes {
			return Checkpoint{}, fmt.Errorf("replay journal reader: payload %d exceeds limit %d", payloadLength, MaxJournalPayloadBytes)
		}
		totalLength := 2 + 8 + 4 + int(payloadLength) + 4
		recordBytes := make([]byte, totalLength)
		copy(recordBytes, header)
		read, err = io.ReadFull(reader, recordBytes[len(header):])
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				_ = read
				break
			}
			return Checkpoint{}, fmt.Errorf("replay journal reader: read record payload: %w", err)
		}
		record, _, err := UnmarshalJournalRecord(recordBytes)
		if err != nil {
			return Checkpoint{}, err
		}
		if record.Sequence != expectedSequence {
			return Checkpoint{}, errors.New("replay journal reader: non-monotonic sequence")
		}
		expectedSequence++
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

// ReplayJournalReaderWithinCapacity combines bounded stream replay with
// selected-media geometry validation.
func ReplayJournalReaderWithinCapacity(checkpoint Checkpoint, reader io.Reader, expectedSectorCount uint64) (Checkpoint, error) {
	replayed, err := ReplayJournalReader(checkpoint, reader)
	if err != nil {
		return Checkpoint{}, err
	}
	if err := ValidateExtentSetWithinCapacity(replayed.Extents, expectedSectorCount); err != nil {
		return Checkpoint{}, fmt.Errorf("replay journal capacity: %w", err)
	}
	return replayed, nil
}
