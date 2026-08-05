package linux

import (
	"encoding/hex"
	"testing"

	"discrescue/internal/device"
)

func TestBuildCDBReadBlocksMatchesFixedVector(t *testing.T) {
	cdb, err := BuildCDB(device.CommandReadBlocks, device.CommandRequest{
		Command:  device.CommandReadBlocks,
		StartLBA: 0x1234,
		Sectors:  0x20,
	})
	if err != nil {
		t.Fatalf("build cdb: %v", err)
	}
	if got, want := hex.EncodeToString(cdb), "28000000123400002000"; got != want {
		t.Fatalf("unexpected read blocks cdb: got %s want %s", got, want)
	}
}

func TestBuildCDBReadCDMatchesFixedVector(t *testing.T) {
	cdb, err := BuildCDB(device.CommandReadCD, device.CommandRequest{
		Command:  device.CommandReadCD,
		StartLBA: 0x1234,
		Sectors:  0x03,
	})
	if err != nil {
		t.Fatalf("build cdb: %v", err)
	}
	if got, want := hex.EncodeToString(cdb), "be0000001234000003100000"; got != want {
		t.Fatalf("unexpected read cd cdb: got %s want %s", got, want)
	}
}

func TestBuildCDBRejectsMissingSectorCount(t *testing.T) {
	if _, err := BuildCDB(device.CommandReadBlocks, device.CommandRequest{
		Command: device.CommandReadBlocks,
	}); err == nil {
		t.Fatal("expected read blocks without sector count to fail")
	}
}

func TestBuildCDBRejectsReadBlocksCountAboveRead10Limit(t *testing.T) {
	if _, err := BuildCDB(device.CommandReadBlocks, device.CommandRequest{
		Command: device.CommandReadBlocks,
		Sectors: 0x10000,
	}); err == nil {
		t.Fatal("expected oversized read blocks request to fail")
	}
}

func TestBuildCDBRejectsReadCDCountAboveLimit(t *testing.T) {
	if _, err := BuildCDB(device.CommandReadCD, device.CommandRequest{
		Command: device.CommandReadCD,
		Sectors: 0x1000000,
	}); err == nil {
		t.Fatal("expected oversized read cd request to fail")
	}
}

func TestBuildCDBInquiryMatchesFixedVector(t *testing.T) {
	cdb, err := BuildCDB(device.CommandInquiry, device.CommandRequest{Command: device.CommandInquiry})
	if err != nil {
		t.Fatalf("build cdb: %v", err)
	}
	if got, want := hex.EncodeToString(cdb), "120000002400"; got != want {
		t.Fatalf("unexpected inquiry cdb: got %s want %s", got, want)
	}
}
