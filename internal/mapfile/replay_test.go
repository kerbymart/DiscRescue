package mapfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"testing"
)

func TestHeaderRoundTrip(t *testing.T) {
	header := Header{
		LogicalSectorSize:        2048,
		ExpectedSectorCount:      4096,
		OutputFormat:             1,
		IdentityAlgorithmVersion: 1,
		CreationUnixNano:         123456789,
		CleanShutdown:            true,
	}

	encoded, err := MarshalHeader(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	decoded, err := UnmarshalHeader(encoded)
	if err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if decoded.LogicalSectorSize != header.LogicalSectorSize || decoded.ExpectedSectorCount != header.ExpectedSectorCount || decoded.CleanShutdown != header.CleanShutdown {
		t.Fatalf("unexpected decoded header: %+v", decoded)
	}
}

func TestCheckpointRoundTrip(t *testing.T) {
	checkpoint := Checkpoint{
		LastSequence: 3,
		Extents: []Extent{
			{StartLBA: 0, Sectors: 4, State: SectorStateVerified, Confidence: ConfidenceRepeatedSingleCapture},
		},
	}

	encoded, err := MarshalCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}
	decoded, err := UnmarshalCheckpoint(encoded)
	if err != nil {
		t.Fatalf("unmarshal checkpoint: %v", err)
	}
	if decoded.LastSequence != checkpoint.LastSequence || len(decoded.Extents) != 1 || decoded.Extents[0].State != checkpoint.Extents[0].State {
		t.Fatalf("unexpected decoded checkpoint: %+v", decoded)
	}
}

func TestUnmarshalCheckpointRejectsCountPayloadMismatchBeforeAllocation(t *testing.T) {
	encoded, err := MarshalCheckpoint(Checkpoint{LastSequence: 1})
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(encoded[18:22], 1)
	if _, err := UnmarshalCheckpoint(encoded); err == nil {
		t.Fatal("expected count/payload mismatch to fail")
	}
}

func TestUnmarshalCheckpointRejectsPayloadLimit(t *testing.T) {
	encoded, err := MarshalCheckpoint(Checkpoint{
		LastSequence: 1,
		Extents: []Extent{{
			StartLBA: 1,
			Sectors:  1,
			State:    SectorStateMissing,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnmarshalCheckpointWithLimits(encoded, DecodeLimits{
		MaxCheckpointPayloadBytes: checkpointFixedPayloadBytes,
		MaxCheckpointExtents:      1,
	}); err == nil {
		t.Fatal("expected checkpoint payload limit to fail")
	}
}

func TestUnmarshalCheckpointRejectsHugeDeclaredCountWithoutAllocating(t *testing.T) {
	encoded := make([]byte, 26)
	copy(encoded[:4], checkpointMagic)
	binary.LittleEndian.PutUint16(encoded[4:6], FormatVersion)
	binary.LittleEndian.PutUint32(encoded[6:10], 12)
	binary.LittleEndian.PutUint32(encoded[18:22], ^uint32(0))
	if _, err := UnmarshalCheckpoint(encoded); err == nil {
		t.Fatal("expected huge count mismatch to fail")
	}
}

func TestReplayJournalIgnoresTruncatedFinalRecord(t *testing.T) {
	extent := Extent{StartLBA: 8, Sectors: 2, State: SectorStateMissing, Confidence: ConfidenceNone}
	record, err := MarshalJournalRecord(JournalRecord{
		Type:     RecordExtentStateChanged,
		Sequence: 1,
		Extent:   &extent,
	})
	if err != nil {
		t.Fatalf("marshal journal record: %v", err)
	}

	truncated := record[:len(record)-3]
	checkpoint, err := ReplayJournal(Checkpoint{}, truncated)
	if err != nil {
		t.Fatalf("replay journal: %v", err)
	}
	if checkpoint.LastSequence != 0 || len(checkpoint.Extents) != 0 {
		t.Fatalf("expected truncated final record to be ignored, got %+v", checkpoint)
	}
}

func TestReplayJournalIgnoresEveryShortFinalHeader(t *testing.T) {
	for size := 1; size < 18; size++ {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			checkpoint, err := ReplayJournal(Checkpoint{}, bytes.Repeat([]byte{0xA5}, size))
			if err != nil {
				t.Fatalf("replay journal with %d-byte tail: %v", size, err)
			}
			if checkpoint.LastSequence != 0 || len(checkpoint.Extents) != 0 {
				t.Fatalf("unexpected state after %d-byte tail: %+v", size, checkpoint)
			}
		})
	}
}

func TestUnmarshalJournalRecordClassifiesShortHeaderAsTruncated(t *testing.T) {
	_, _, err := UnmarshalJournalRecord(make([]byte, 17))
	if !errors.Is(err, ErrTruncatedRecord) {
		t.Fatalf("error = %v, want ErrTruncatedRecord", err)
	}
}

func TestReplayJournalRejectsCompleteCorruptRecord(t *testing.T) {
	record, err := MarshalJournalRecord(JournalRecord{Type: RecordJobCreated, Sequence: 1})
	if err != nil {
		t.Fatal(err)
	}
	record[len(record)-1] ^= 0xFF
	if _, err := ReplayJournal(Checkpoint{}, record); err == nil {
		t.Fatal("expected complete record corruption to fail")
	}
}

func TestReplayJournalRevertsQueuedStateToUnknown(t *testing.T) {
	extent := Extent{StartLBA: 4, Sectors: 2, State: SectorStateQueued, Confidence: ConfidenceNone}
	journal, err := AppendJournalRecord(nil, JournalRecord{
		Type:     RecordExtentStateChanged,
		Sequence: 1,
		Extent:   &extent,
	})
	if err != nil {
		t.Fatalf("append journal record: %v", err)
	}

	checkpoint, err := ReplayJournal(Checkpoint{}, journal)
	if err != nil {
		t.Fatalf("replay journal: %v", err)
	}
	if len(checkpoint.Extents) != 1 {
		t.Fatalf("expected one replayed extent, got %d", len(checkpoint.Extents))
	}
	if checkpoint.Extents[0].State != SectorStateUnknown || checkpoint.Extents[0].Confidence != ConfidenceNone {
		t.Fatalf("expected queued state to revert to unknown, got %+v", checkpoint.Extents[0])
	}
}

func TestReplayJournalAppliesExtentRecord(t *testing.T) {
	base := Checkpoint{
		LastSequence: 1,
		Extents: []Extent{
			{StartLBA: 0, Sectors: 4, State: SectorStateUnknown, Confidence: ConfidenceNone},
		},
	}
	extent := Extent{StartLBA: 4, Sectors: 2, State: SectorStateMissing, Confidence: ConfidenceNone}
	journal, err := AppendJournalRecord(nil, JournalRecord{
		Type:     RecordExtentStateChanged,
		Sequence: 2,
		Extent:   &extent,
	})
	if err != nil {
		t.Fatalf("append journal record: %v", err)
	}

	checkpoint, err := ReplayJournal(base, journal)
	if err != nil {
		t.Fatalf("replay journal: %v", err)
	}
	if checkpoint.LastSequence != 2 || len(checkpoint.Extents) != 2 {
		t.Fatalf("unexpected replayed checkpoint: %+v", checkpoint)
	}
}

func TestReplayJournalAppliesStateRefinement(t *testing.T) {
	unknown := Extent{StartLBA: 64, Sectors: 16, State: SectorStateUnknown, Confidence: ConfidenceNone, Attempts: 1}
	queued := Extent{StartLBA: 64, Sectors: 16, State: SectorStateQueued, Confidence: ConfidenceNone, Attempts: 2}
	recovered := Extent{StartLBA: 64, Sectors: 16, State: SectorStateReadUnverified, Confidence: ConfidenceSingleRead, Attempts: 2}

	journal := []byte{}
	var err error
	for sequence, extent := range []Extent{unknown, queued, recovered} {
		candidate := extent
		journal, err = AppendJournalRecord(journal, JournalRecord{
			Type:     RecordExtentStateChanged,
			Sequence: uint64(sequence + 1),
			Extent:   &candidate,
		})
		if err != nil {
			t.Fatalf("append journal record %d: %v", sequence+1, err)
		}
	}

	checkpoint, err := ReplayJournal(Checkpoint{}, journal)
	if err != nil {
		t.Fatalf("replay journal: %v", err)
	}
	if checkpoint.LastSequence != 3 || len(checkpoint.Extents) != 1 {
		t.Fatalf("unexpected replayed checkpoint: %+v", checkpoint)
	}
	if checkpoint.Extents[0].State != SectorStateReadUnverified || checkpoint.Extents[0].Attempts != 2 {
		t.Fatalf("expected refined recovered extent, got %+v", checkpoint.Extents[0])
	}
}
