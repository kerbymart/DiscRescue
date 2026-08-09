package platform

import (
	"strings"
	"testing"

	"discrescue/internal/catalog"
)

func TestRecoveryMapHeaderBindsProvidedIdentity(t *testing.T) {
	identity := catalog.ContentIdentity{
		Version:          1,
		LogicalBlockSize: 2048,
		SectorCount:      100,
		LayoutSHA256:     strings.Repeat("ab", 32),
		QuickID:          strings.Repeat("cd", 32),
	}
	header, err := recoveryMapHeader(RecoveryInput{
		LogicalSectorSize: 2048,
		CapacitySectors:   100,
		Identity:          &identity,
		CaptureID:         catalog.RecordID{1},
		JobID:             catalog.RecordID{2},
		CatalogRecordID:   catalog.RecordID{3},
	})
	if err != nil {
		t.Fatalf("recoveryMapHeader() error = %v", err)
	}
	if header.IdentityAlgorithmVersion != 1 || !header.QuickContentIDPresent || header.QuickContentID[0] != 0xcd || header.LayoutSHA256[0] != 0xab {
		t.Fatalf("identity binding missing from map header: %+v", header)
	}
}
