package catalog

import (
	"encoding/hex"
	"testing"
)

func TestCoreCodecGoldenBytes(t *testing.T) {
	snapshot, err := MarshalSnapshot(Snapshot{LastSequence: 1})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if got, want := hex.EncodeToString(snapshot), "445343530100000000000100000000000000000000002b9ceab8"; got != want {
		t.Fatalf("unexpected snapshot bytes: got %s want %s", got, want)
	}

	record, err := MarshalJournalRecord(JournalRecord{Type: EventMediaObserved, Sequence: 1})
	if err != nil {
		t.Fatalf("marshal journal: %v", err)
	}
	if got, want := hex.EncodeToString(record), "4453434a01000100010000000000000000000000cf4eeb97"; got != want {
		t.Fatalf("unexpected journal bytes: got %s want %s", got, want)
	}
}
