package mapfile

import "testing"

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

func TestReplayJournalReplacesOverlappingExtentRecord(t *testing.T) {
	base := Checkpoint{
		LastSequence: 1,
		Extents: []Extent{
			{StartLBA: 0, Sectors: 8, State: SectorStateSkipped, Confidence: ConfidenceNone},
		},
	}
	extent := Extent{StartLBA: 3, Sectors: 2, State: SectorStateMissing, Confidence: ConfidenceNone}
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
	if len(checkpoint.Extents) != 3 {
		t.Fatalf("expected replacement extents, got %+v", checkpoint.Extents)
	}
	if checkpoint.Extents[1].StartLBA != 3 || checkpoint.Extents[1].Sectors != 2 || checkpoint.Extents[1].State != SectorStateMissing {
		t.Fatalf("unexpected replacement extent: %+v", checkpoint.Extents[1])
	}
}
