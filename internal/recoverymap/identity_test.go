package recoverymap

import (
	"strings"
	"testing"

	"discrescue/internal/catalog"
	"discrescue/internal/mapfile"
)

func TestIdentityBindingUsesTruncatedQuickIDAndPreservesProvenance(t *testing.T) {
	identity := testIdentity("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	capture := catalog.RecordID{1}
	job := catalog.RecordID{2}
	record := catalog.RecordID{3}
	header, err := IdentityBinding(mapfile.Header{LogicalSectorSize: 2048, ExpectedSectorCount: 100}, identity, capture, job, record)
	if err != nil {
		t.Fatalf("IdentityBinding() error = %v", err)
	}
	if !header.QuickContentIDPresent || header.QuickContentID[0] != 0x00 || header.QuickContentID[15] != 0xff {
		t.Fatalf("unexpected compact QuickID binding: %x", header.QuickContentID)
	}
	if header.CaptureID != capture || header.JobID != job || header.CatalogRecordID != record || !header.CatalogRecordIDPresent {
		t.Fatalf("provenance binding was not preserved: %+v", header)
	}
	if err := ValidateIdentityBinding(header, identity); err != nil {
		t.Fatalf("ValidateIdentityBinding() error = %v", err)
	}

	changed := identity
	changed.QuickID = strings.Repeat("00", 32)
	if err := ValidateIdentityBinding(header, changed); err == nil {
		t.Fatal("ValidateIdentityBinding() accepted a mismatched QuickID")
	}
}

func testIdentity(layout, quick string) catalog.ContentIdentity {
	return catalog.ContentIdentity{
		Version:          1,
		LogicalBlockSize: 2048,
		SectorCount:      100,
		LayoutSHA256:     layout,
		QuickID:          quick,
	}
}
