package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"discrescue/internal/catalog"
	"discrescue/internal/mapfile"
)

func TestVerifyMapReplaysJournalAndChecksImageLength(t *testing.T) {
	headerBytes, checkpointBytes, journalBytes := verificationMapFixture(t)

	result, err := VerifyMap(MapVerificationInput{
		HeaderBytes:     headerBytes,
		CheckpointBytes: checkpointBytes,
		JournalBytes:    journalBytes,
		ImageLength:     4 * 2048,
	})
	if err != nil {
		t.Fatalf("verify map: %v", err)
	}

	if result.Header.LogicalSectorSize != 2048 {
		t.Fatalf("unexpected header: %+v", result.Header)
	}
	if len(result.ReplayedCheckpoint.Extents) != 1 {
		t.Fatalf("unexpected replayed extents: %+v", result.ReplayedCheckpoint.Extents)
	}
	if result.RequiredImageBytes != 4*2048 {
		t.Fatalf("unexpected required image bytes: %d", result.RequiredImageBytes)
	}
}

func TestVerifyImageDowngradesMismatchedExtent(t *testing.T) {
	data := bytesForSectors(2, 2048, 'a')
	extent := mapfile.Extent{
		StartLBA:   4,
		Sectors:    2,
		State:      mapfile.SectorStateVerified,
		Confidence: mapfile.ConfidenceTrustedChecksum,
		DataHash:   hash16(data),
	}

	result, err := VerifyImage(ImageVerificationInput{
		LogicalSectorSize: 2048,
		Extents: []ImageExtentInput{{
			Extent: extent,
			Data:   bytesForSectors(2, 2048, 'b'),
		}},
	})
	if err != nil {
		t.Fatalf("verify image: %v", err)
	}

	if result.Extents[0].State != mapfile.SectorStateChecksumError {
		t.Fatalf("unexpected downgraded state: %+v", result.Extents[0])
	}
	if len(result.ChangedRanges) != 1 || result.ChangedRanges[0].Offset != 4*2048 {
		t.Fatalf("unexpected changed ranges: %+v", result.ChangedRanges)
	}
	if len(result.Provenance) != 1 || len(result.Provenance[0].Items) != 0 {
		t.Fatalf("expected downgraded checksum mismatch to clear provenance, got %+v", result.Provenance)
	}
}

func TestVerifyExternalPromotesTrustedAndReconstructedMatches(t *testing.T) {
	verifiedData := bytesForSectors(1, 2048, 'v')
	reconstructedData := bytesForSectors(1, 2048, 'r')

	verifiedSum := sha256.Sum256(verifiedData)
	reconstructedSum := sha256.Sum256(reconstructedData)

	result, err := VerifyExternal(ExternalVerificationInput{
		LogicalSectorSize: 2048,
		Extents: []ExternalExtentInput{
			{
				Extent: mapfile.Extent{
					StartLBA:   0,
					Sectors:    1,
					State:      mapfile.SectorStateReadUnverified,
					Confidence: mapfile.ConfidenceSingleRead,
				},
				Data: verifiedData,
				Digests: []Digest{{
					Provider: ProviderSHA256,
					Value:    hex.EncodeToString(verifiedSum[:]),
				}},
			},
			{
				Extent: mapfile.Extent{
					StartLBA:   1,
					Sectors:    1,
					State:      mapfile.SectorStateReconstructed,
					Confidence: mapfile.ConfidenceReconstructedChecksum,
				},
				Data: reconstructedData,
				Digests: []Digest{{
					Provider: ProviderSHA256,
					Value:    hex.EncodeToString(reconstructedSum[:]),
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("verify external: %v", err)
	}

	if result.Extents[0].State != mapfile.SectorStateVerified || result.Extents[0].Confidence != mapfile.ConfidenceTrustedChecksum {
		t.Fatalf("unexpected trusted verification result: %+v", result.Extents[0])
	}
	if result.Extents[1].State != mapfile.SectorStateReconstructed || result.Extents[1].Confidence != mapfile.ConfidenceReconstructedChecksum {
		t.Fatalf("unexpected reconstructed verification result: %+v", result.Extents[1])
	}
	if !reflect.DeepEqual(result.Provenance[0].Items, []Evidence{EvidenceTrustedChecksum}) {
		t.Fatalf("unexpected trusted checksum provenance: %+v", result.Provenance[0])
	}
	if !reflect.DeepEqual(result.Provenance[1].Items, []Evidence{EvidenceReconstruction, EvidenceTrustedChecksum}) {
		t.Fatalf("unexpected reconstructed provenance: %+v", result.Provenance[1])
	}
}

func TestRefreshCatalogEntryUpdatesPathStateAndFullHash(t *testing.T) {
	entry := catalog.Entry{
		RecordID:          mustRecordID(1),
		Identity:          verificationIdentity("layout-a"),
		State:             catalog.ProcessingObserved,
		Status:            string(catalog.ProcessingObserved),
		FirstSeenUnixNano: time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC).UnixNano(),
		LastSeenUnixNano:  time.Date(2026, time.August, 5, 10, 5, 0, 0, time.UTC).UnixNano(),
		JobReferences: []catalog.JobReference{{
			JobID:        mustRecordID(2),
			Path:         "D:/Archives/archive-disc.iso",
			FilesPresent: true,
		}},
	}

	updated, err := RefreshCatalogEntry(CatalogRefreshInput{
		Entry:             entry,
		FilesPresentByJob: map[catalog.RecordID]bool{mustRecordID(2): false},
		State:             catalog.ProcessingCompletedVerified,
		FullContentSHA256: "full-content-hash",
	})
	if err != nil {
		t.Fatalf("refresh catalog entry: %v", err)
	}

	if updated.State != catalog.ProcessingCompletedVerified {
		t.Fatalf("unexpected updated state: %+v", updated)
	}
	if updated.Identity.FullContentSHA256 != "full-content-hash" {
		t.Fatalf("unexpected full-content hash: %+v", updated.Identity)
	}
	if updated.JobReferences[0].FilesPresent {
		t.Fatalf("expected file presence refresh to be applied: %+v", updated.JobReferences)
	}
}

func TestEvaluateFilesystemAdvisoriesKeepsExtentsUnchanged(t *testing.T) {
	extents := []mapfile.Extent{{
		StartLBA:   0,
		Sectors:    1,
		State:      mapfile.SectorStateReadUnverified,
		Confidence: mapfile.ConfidenceSingleRead,
	}}

	result, err := EvaluateFilesystemAdvisories([]FilesystemAdvisory{{
		Code:   "iso9660_suspicious_root",
		Detail: "primary volume descriptor points past the last recorded area",
	}}, extents)
	if err != nil {
		t.Fatalf("evaluate filesystem advisories: %v", err)
	}

	if len(result.Extents) != 1 || result.Extents[0].State != mapfile.SectorStateReadUnverified {
		t.Fatalf("expected advisory-only result, got %+v", result.Extents)
	}
}

func verificationMapFixture(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()

	headerBytes, err := mapfile.MarshalHeader(mapfile.Header{
		LogicalSectorSize:        2048,
		ExpectedSectorCount:      16,
		OutputFormat:             1,
		IdentityAlgorithmVersion: 1,
		CleanShutdown:            true,
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}

	checkpointBytes, err := mapfile.MarshalCheckpoint(mapfile.Checkpoint{})
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}

	extent := mapfile.Extent{
		StartLBA:   0,
		Sectors:    4,
		State:      mapfile.SectorStateReadUnverified,
		Confidence: mapfile.ConfidenceSingleRead,
	}
	recordBytes, err := mapfile.MarshalJournalRecord(mapfile.JournalRecord{
		Sequence: 1,
		Type:     mapfile.RecordExtentStateChanged,
		Extent:   &extent,
	})
	if err != nil {
		t.Fatalf("marshal journal record: %v", err)
	}
	return headerBytes, checkpointBytes, recordBytes
}

func verificationIdentity(layout string) catalog.ContentIdentity {
	return catalog.ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4096,
		Sessions:         1,
		Tracks: []catalog.TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
		},
		LayoutSHA256: layout,
	}
}

func mustRecordID(seed byte) catalog.RecordID {
	var id catalog.RecordID
	for i := range id {
		id[i] = seed
	}
	return id
}

func bytesForSectors(sectors int, sectorSize int, fill byte) []byte {
	data := make([]byte, sectors*sectorSize)
	for i := range data {
		data[i] = fill
	}
	return data
}
