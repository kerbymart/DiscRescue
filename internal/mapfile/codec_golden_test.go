package mapfile

import (
	"encoding/hex"
	"testing"
)

func TestCoreCodecGoldenBytes(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		want string
	}{
		{
			name: "header",
			got:  mustMarshalHeaderForTest(t, Header{LogicalSectorSize: 2048, ExpectedSectorCount: 4}),
			want: "4453523101008700000800000400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000d7d7ae6c",
		},
		{
			name: "checkpoint",
			got:  mustMarshalCheckpointForTest(t, Checkpoint{LastSequence: 1}),
			want: "4453435001000c0000000100000000000000000000003fbcaa39",
		},
		{
			name: "journal",
			got:  mustMarshalJournalForTest(t, JournalRecord{Type: RecordJobCreated, Sequence: 1}),
			want: "010001000000000000000000000087b61d13",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hex.EncodeToString(test.got); got != test.want {
				t.Fatalf("unexpected %s bytes: got %s want %s", test.name, got, test.want)
			}
		})
	}
}

func mustMarshalHeaderForTest(t *testing.T, header Header) []byte {
	t.Helper()
	encoded, err := MarshalHeader(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	return encoded
}

func mustMarshalCheckpointForTest(t *testing.T, checkpoint Checkpoint) []byte {
	t.Helper()
	encoded, err := MarshalCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}
	return encoded
}

func mustMarshalJournalForTest(t *testing.T, record JournalRecord) []byte {
	t.Helper()
	encoded, err := MarshalJournalRecord(record)
	if err != nil {
		t.Fatalf("marshal journal: %v", err)
	}
	return encoded
}
