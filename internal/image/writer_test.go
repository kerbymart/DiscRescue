package image

import (
	"bytes"
	"testing"
)

func TestSectorWriteOffsetUsesLBAAndSectorSize(t *testing.T) {
	write := SectorWrite{
		LBA:               12,
		LogicalSectorSize: 2048,
		Data:              make([]byte, 2048),
	}

	if got := write.Offset(); got != 12*2048 {
		t.Fatalf("unexpected offset: got %d want %d", got, 12*2048)
	}
}

func TestSectorWriteValidateRejectsPartialSector(t *testing.T) {
	write := SectorWrite{
		LBA:               5,
		LogicalSectorSize: 2048,
		Data:              make([]byte, 1024),
	}

	if err := write.Validate(); err == nil {
		t.Fatal("expected partial sector write to fail validation")
	}
}

func TestBuildPositionedWritesCoalescesContiguousWrites(t *testing.T) {
	first := bytes.Repeat([]byte{0x11}, 2048)
	second := bytes.Repeat([]byte{0x22}, 2048)

	positioned, err := BuildPositionedWrites(WriterPlan{
		LogicalSectorSize: 2048,
		ExpectedSectors:   8,
		Writes: []SectorWrite{
			{LBA: 0, LogicalSectorSize: 2048, Data: first},
			{LBA: 1, LogicalSectorSize: 2048, Data: second},
		},
	})
	if err != nil {
		t.Fatalf("build positioned writes: %v", err)
	}
	if len(positioned) != 1 {
		t.Fatalf("expected one coalesced positioned write, got %d", len(positioned))
	}
	if positioned[0].Offset != 0 {
		t.Fatalf("unexpected positioned offset: %d", positioned[0].Offset)
	}
	if len(positioned[0].Data) != 4096 {
		t.Fatalf("unexpected coalesced data length: %d", len(positioned[0].Data))
	}
}

func TestBuildPositionedWritesSplitsNonContiguousWrites(t *testing.T) {
	positioned, err := BuildPositionedWrites(WriterPlan{
		LogicalSectorSize: 2048,
		ExpectedSectors:   8,
		Writes: []SectorWrite{
			{LBA: 0, LogicalSectorSize: 2048, Data: make([]byte, 2048)},
			{LBA: 2, LogicalSectorSize: 2048, Data: make([]byte, 2048)},
		},
	})
	if err != nil {
		t.Fatalf("build positioned writes: %v", err)
	}
	if len(positioned) != 2 {
		t.Fatalf("expected two positioned writes, got %d", len(positioned))
	}
	if positioned[1].Offset != 4096 {
		t.Fatalf("unexpected second positioned offset: %d", positioned[1].Offset)
	}
}

func TestWriterPlanRejectsOutOfBoundsLBA(t *testing.T) {
	_, err := BuildPositionedWrites(WriterPlan{
		LogicalSectorSize: 2048,
		ExpectedSectors:   2,
		Writes: []SectorWrite{
			{LBA: 2, LogicalSectorSize: 2048, Data: make([]byte, 2048)},
		},
	})
	if err == nil {
		t.Fatal("expected out-of-bounds lba to fail")
	}
}
