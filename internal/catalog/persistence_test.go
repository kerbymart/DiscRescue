package catalog

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestMarshalSnapshotRoundTrip(t *testing.T) {
	snapshot := Snapshot{
		LastSequence: 9,
		Entries: []Entry{
			testCatalogEntry(1, 100, 200),
			testCatalogEntry(2, 300, 400),
		},
	}

	encoded, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	decoded, err := UnmarshalSnapshot(encoded)
	if err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	snapshot = normalizedPersistedSnapshot(snapshot)
	if !reflect.DeepEqual(decoded, snapshot) {
		t.Fatalf("snapshot mismatch:\nwant: %#v\ngot:  %#v", snapshot, decoded)
	}
}

func TestJournalRecordRoundTrip(t *testing.T) {
	event := CatalogEvent{
		Type:  EventJobStateChanged,
		Entry: testCatalogEntry(3, 500, 600),
	}

	record, err := EncodeCatalogEvent(event, 7)
	if err != nil {
		t.Fatalf("encode catalog event: %v", err)
	}

	encoded, err := MarshalJournalRecord(record)
	if err != nil {
		t.Fatalf("marshal journal record: %v", err)
	}

	decodedRecord, consumed, err := UnmarshalJournalRecord(encoded)
	if err != nil {
		t.Fatalf("unmarshal journal record: %v", err)
	}
	if consumed != len(encoded) {
		t.Fatalf("expected to consume %d bytes, consumed %d", len(encoded), consumed)
	}

	decodedEvent, err := DecodeCatalogEvent(decodedRecord)
	if err != nil {
		t.Fatalf("decode catalog event: %v", err)
	}

	event = normalizedPersistedEvent(event)
	if !reflect.DeepEqual(decodedEvent, event) {
		t.Fatalf("event mismatch:\nwant: %#v\ngot:  %#v", event, decodedEvent)
	}
}

func TestReplayJournalSkipsSnapshotHistoryAndIgnoresTruncatedTail(t *testing.T) {
	snapshot := Snapshot{
		LastSequence: 2,
		Entries: []Entry{
			testCatalogEntry(1, 100, 100),
		},
	}

	record2, err := EncodeCatalogEvent(CatalogEvent{
		Type:  EventCaptureStarted,
		Entry: testCatalogEntry(2, 200, 200),
	}, 2)
	if err != nil {
		t.Fatalf("encode sequence 2: %v", err)
	}
	record3, err := EncodeCatalogEvent(CatalogEvent{
		Type:  EventJobLinked,
		Entry: testCatalogEntry(3, 300, 300),
	}, 3)
	if err != nil {
		t.Fatalf("encode sequence 3: %v", err)
	}
	record4, err := EncodeCatalogEvent(CatalogEvent{
		Type:  EventJobStateChanged,
		Entry: testCatalogEntry(4, 400, 400),
	}, 4)
	if err != nil {
		t.Fatalf("encode sequence 4: %v", err)
	}

	journal, err := AppendJournal(nil, record2)
	if err != nil {
		t.Fatalf("append sequence 2: %v", err)
	}
	journal, err = AppendJournal(journal, record3)
	if err != nil {
		t.Fatalf("append sequence 3: %v", err)
	}
	journal, err = AppendJournal(journal, record4)
	if err != nil {
		t.Fatalf("append sequence 4: %v", err)
	}
	journal = journal[:len(journal)-5]

	store, err := ReplayJournal(snapshot, journal)
	if err != nil {
		t.Fatalf("replay journal: %v", err)
	}

	if store.LastSequence != 3 {
		t.Fatalf("expected last sequence 3, got %d", store.LastSequence)
	}
	if len(store.Entries) != 2 {
		t.Fatalf("expected 2 entries after replay, got %d", len(store.Entries))
	}
	if store.Entries[0].RecordID != testCatalogEntry(1, 100, 100).RecordID {
		t.Fatalf("expected snapshot entry to remain first, got %#v", store.Entries[0].RecordID)
	}
	if store.Entries[1].RecordID != testCatalogEntry(3, 300, 300).RecordID {
		t.Fatalf("expected sequence 3 entry to replay, got %#v", store.Entries[1].RecordID)
	}
}

func TestStoreCompactAppliesEntryCaptureAndJobReferenceBounds(t *testing.T) {
	store := Store{
		LastSequence: 11,
		Entries: []Entry{
			testCatalogEntry(1, 100, 100),
			testCatalogEntry(2, 100, 300),
			testCatalogEntry(3, 100, 200),
		},
	}

	compacted, err := store.Compact(Bounds{
		MaxEntries:          2,
		MaxCapturesPerEntry: 1,
		MaxJobRefsPerEntry:  1,
	})
	if err != nil {
		t.Fatalf("compact store: %v", err)
	}

	if len(compacted.Entries) != 2 {
		t.Fatalf("expected 2 entries after compaction, got %d", len(compacted.Entries))
	}
	if compacted.Entries[0].RecordID != testCatalogEntry(2, 100, 300).RecordID {
		t.Fatalf("expected newest entry first, got %#v", compacted.Entries[0].RecordID)
	}
	if compacted.Entries[1].RecordID != testCatalogEntry(3, 100, 200).RecordID {
		t.Fatalf("expected second-newest entry second, got %#v", compacted.Entries[1].RecordID)
	}
	if len(compacted.Entries[0].Captures) != 1 || compacted.Entries[0].Captures[0].CaptureID != "capture-2-b" {
		t.Fatalf("expected compacted captures to keep newest capture, got %#v", compacted.Entries[0].Captures)
	}
	if len(compacted.Entries[0].JobReferences) != 1 || compacted.Entries[0].JobReferences[0].Path != "D:/archive/job-2-b" {
		t.Fatalf("expected compacted job references to keep newest reference, got %#v", compacted.Entries[0].JobReferences)
	}
}

func TestStoreOpenAllowsReadOnlyDuringContention(t *testing.T) {
	store := Store{MutationLockHeld: true}

	readOnly, err := store.Open(false)
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	if readOnly.Mode != OpenReadOnly {
		t.Fatalf("expected read-only mode, got %q", readOnly.Mode)
	}
	if readOnly.HistoryUpdatesAvailable {
		t.Fatal("expected history updates to be unavailable while mutation lock is held")
	}

	if _, err := store.Open(true); err == nil {
		t.Fatal("expected mutable open to fail while mutation lock is held")
	}
}

func TestTruncateJournalAfterSnapshotDropsCoveredRecords(t *testing.T) {
	record1, err := EncodeCatalogEvent(CatalogEvent{
		Type:  EventMediaObserved,
		Entry: testCatalogEntry(1, 100, 100),
	}, 1)
	if err != nil {
		t.Fatalf("encode sequence 1: %v", err)
	}
	record2, err := EncodeCatalogEvent(CatalogEvent{
		Type:  EventCaptureStarted,
		Entry: testCatalogEntry(2, 200, 200),
	}, 2)
	if err != nil {
		t.Fatalf("encode sequence 2: %v", err)
	}
	record3, err := EncodeCatalogEvent(CatalogEvent{
		Type:  EventJobLinked,
		Entry: testCatalogEntry(3, 300, 300),
	}, 3)
	if err != nil {
		t.Fatalf("encode sequence 3: %v", err)
	}

	journal, err := AppendJournal(nil, record1)
	if err != nil {
		t.Fatalf("append sequence 1: %v", err)
	}
	journal, err = AppendJournal(journal, record2)
	if err != nil {
		t.Fatalf("append sequence 2: %v", err)
	}
	journal, err = AppendJournal(journal, record3)
	if err != nil {
		t.Fatalf("append sequence 3: %v", err)
	}

	truncated := TruncateJournalAfterSnapshot(journal, Snapshot{LastSequence: 2})
	replayed, err := ReplayJournal(Snapshot{LastSequence: 2}, truncated)
	if err != nil {
		t.Fatalf("replay truncated journal: %v", err)
	}

	if replayed.LastSequence != 3 {
		t.Fatalf("expected remaining journal to start at sequence 3, got %d", replayed.LastSequence)
	}
	if len(replayed.Entries) != 1 || replayed.Entries[0].RecordID != testCatalogEntry(3, 300, 300).RecordID {
		t.Fatalf("expected only uncovered entry to remain, got %#v", replayed.Entries)
	}
}

func TestUnmarshalSnapshotRejectsUntrustedEntryCountBeforeAllocation(t *testing.T) {
	snapshot := Snapshot{LastSequence: 1, Entries: []Entry{testCatalogEntry(8, 1, 1)}}
	encoded, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatalf("MarshalSnapshot() error = %v", err)
	}
	// Keep the payload and CRC valid enough to reach the count bounds check.
	encoded[18], encoded[19], encoded[20], encoded[21] = 0xff, 0xff, 0xff, 0x7f
	if _, err := UnmarshalSnapshot(encoded); err == nil {
		t.Fatal("UnmarshalSnapshot() accepted an oversized entry count")
	}
}

func testCatalogEntry(idByte byte, firstSeen, lastSeen int64) Entry {
	recordID := RecordID{idByte}
	preferredJobID := RecordID{idByte + 32}
	jobAID := RecordID{idByte + 64}
	jobBID := RecordID{idByte + 96}

	return Entry{
		RecordID:              recordID,
		Identity:              testContentIdentity(idByte),
		State:                 ProcessingInProgress,
		Status:                string(ProcessingInProgress),
		FirstSeenUnixNano:     firstSeen,
		LastSeenUnixNano:      lastSeen,
		LastProcessedPresent:  true,
		LastProcessedUnixNano: lastSeen,
		Captures: []CaptureIdentity{
			testCaptureIdentity(idByte, "a", firstSeen),
			testCaptureIdentity(idByte, "b", lastSeen),
		},
		JobReferences: []JobReference{
			{JobID: jobAID, Path: testJobPath(idByte, "a"), FilesPresent: false},
			{JobID: jobBID, Path: testJobPath(idByte, "b"), FilesPresent: true},
		},
		PreferredJobID: preferredJobID,
		Hidden:         idByte%2 == 0,
	}
}

func testContentIdentity(idByte byte) ContentIdentity {
	return ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4096,
		Sessions:         1,
		Tracks: []TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
		},
		LayoutSHA256: "layout-hash",
		VolumeHints: []VolumeHint{
			{HintType: 1, Value: "DISCRESCUE"},
		},
		Samples: []SectorFingerprint{
			{
				Slot:      1,
				LBA:       0,
				Available: true,
				SHA256:    sha256.Sum256([]byte{idByte, 1}),
			},
			{
				Slot:      2,
				LBA:       32,
				Available: true,
				SHA256:    sha256.Sum256([]byte{idByte, 2}),
			},
		},
		QuickID:           "quick-id",
		FullContentSHA256: "full-content-hash",
	}
}

func testCaptureIdentity(idByte byte, suffix string, unixNano int64) CaptureIdentity {
	return CaptureIdentity{
		CaptureID: fmt.Sprintf("capture-%d-%s", idByte, suffix),
		Device: DeviceIdentity{
			Vendor:    "ACME",
			Product:   "Optical",
			Revision:  "1.0",
			Serial:    "SERIAL-" + suffix,
			Transport: "usb",
		},
		StartedAt: time.Unix(0, unixNano).UTC(),
		UserLabel: "label-" + suffix,
		PhysicalCopy: &PhysicalCopyIdentity{
			AssetID:     "asset-" + suffix,
			HubCodeNote: "hub-" + suffix,
		},
	}
}

func testJobPath(idByte byte, suffix string) string {
	return fmt.Sprintf("D:/archive/job-%d-%s", idByte, suffix)
}

func normalizedPersistedSnapshot(snapshot Snapshot) Snapshot {
	next := snapshot
	next.Entries = make([]Entry, len(snapshot.Entries))
	for i, entry := range snapshot.Entries {
		next.Entries[i] = normalizedPersistedEntry(entry)
	}
	return next
}

func normalizedPersistedEvent(event CatalogEvent) CatalogEvent {
	event.Entry = normalizedPersistedEntry(event.Entry)
	return event
}

func normalizedPersistedEntry(entry Entry) Entry {
	entry.Status = ""
	return entry
}
