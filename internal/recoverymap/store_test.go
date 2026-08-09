package recoverymap

import (
	"bytes"
	"path/filepath"
	"testing"

	"discrescue/internal/mapfile"
)

func testHeader() mapfile.Header {
	header := mapfile.Header{
		LogicalSectorSize:        2048,
		ExpectedSectorCount:      128,
		OutputFormat:             1,
		IdentityAlgorithmVersion: 7,
		CreationUnixNano:         1234,
		CleanShutdown:            true,
	}
	for i := range header.LayoutSHA256 {
		header.LayoutSHA256[i] = byte(i + 1)
	}
	header.QuickContentIDPresent = true
	for i := range header.QuickContentID {
		header.QuickContentID[i] = byte(16 - i)
	}
	for i := range header.CaptureID {
		header.CaptureID[i] = byte(32 + i)
	}
	header.CatalogRecordIDPresent = true
	for i := range header.CatalogRecordID {
		header.CatalogRecordID[i] = byte(48 + i)
	}
	for i := range header.JobID {
		header.JobID[i] = byte(64 + i)
	}
	return header
}

func TestCreateAndOpenPreservesHeaderMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.drmap")
	created, err := Create(path, testHeader())
	if err != nil {
		t.Fatal(err)
	}
	if created.Header().CleanShutdown {
		t.Fatal("new session must not be clean")
	}
	if err := created.Close(true); err != nil {
		t.Fatal(err)
	}

	resumed, err := Open(path, Geometry{LogicalSectorSize: 2048, ExpectedSectorCount: 128})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Header().CleanShutdown {
		t.Fatal("opened writable session must be marked unclean")
	}
	header := resumed.Header()
	expected := testHeader()
	if header.IdentityAlgorithmVersion != expected.IdentityAlgorithmVersion || header.CreationUnixNano != expected.CreationUnixNano || header.OutputFormat != expected.OutputFormat ||
		header.LogicalSectorSize != expected.LogicalSectorSize || header.ExpectedSectorCount != expected.ExpectedSectorCount ||
		header.QuickContentIDPresent != expected.QuickContentIDPresent || header.CatalogRecordIDPresent != expected.CatalogRecordIDPresent ||
		header.LayoutSHA256 != expected.LayoutSHA256 || header.QuickContentID != expected.QuickContentID || header.CaptureID != expected.CaptureID ||
		header.CatalogRecordID != expected.CatalogRecordID || header.JobID != expected.JobID {
		t.Fatalf("metadata changed during resume: %+v", header)
	}
	if !bytes.Equal(header.LayoutSHA256[:], expected.LayoutSHA256[:]) {
		t.Fatal("layout identity changed during resume")
	}
	extent := mapfile.Extent{StartLBA: 8, Sectors: 2, State: mapfile.SectorStateMissing}
	if err := resumed.ApplyExtent(extent); err != nil {
		t.Fatal(err)
	}
	if err := resumed.Close(true); err != nil {
		t.Fatal(err)
	}

	final, err := Open(path, Geometry{LogicalSectorSize: 2048, ExpectedSectorCount: 128})
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Extents()) != 1 || final.Extents()[0].StartLBA != 8 {
		t.Fatalf("unexpected resumed extents: %+v", final.Extents())
	}
	if err := final.Close(false); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsMapExtentOutsideGeometry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.drmap")
	created, err := Create(path, testHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err := created.ApplyExtent(mapfile.Extent{StartLBA: 127, Sectors: 1, State: mapfile.SectorStateMissing}); err != nil {
		t.Fatal(err)
	}
	if err := created.Close(true); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, Geometry{LogicalSectorSize: 2048, ExpectedSectorCount: 64}); err == nil {
		t.Fatal("expected geometry mismatch to fail")
	}
}

func TestStageExtentFlushesAtBoundedRecordCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.drmap")
	header := testHeader()
	header.ExpectedSectorCount = 512
	store, err := Create(path, header)
	if err != nil {
		t.Fatal(err)
	}
	for lba := uint64(0); lba < 256; lba++ {
		if err := store.StageExtent(mapfile.Extent{StartLBA: lba, Sectors: 1, State: mapfile.SectorStateMissing}); err != nil {
			t.Fatal(err)
		}
	}
	if pending := store.PendingBytes(); pending != 0 {
		t.Fatalf("expected bounded batch to flush at record limit, %d bytes remain", pending)
	}
	if err := store.Close(true); err != nil {
		t.Fatal(err)
	}
}

func TestDurableExtentsLagUntilBatchFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.drmap")
	store, err := Create(path, testHeader())
	if err != nil {
		t.Fatal(err)
	}
	extent := mapfile.Extent{StartLBA: 8, Sectors: 2, State: mapfile.SectorStateReadUnverified, Confidence: mapfile.ConfidenceSingleRead}
	if err := store.StageExtent(extent); err != nil {
		t.Fatal(err)
	}
	if len(store.Extents()) != 1 || len(store.DurableExtents()) != 0 {
		t.Fatalf("working/durable extents = %d/%d, want 1/0", len(store.Extents()), len(store.DurableExtents()))
	}
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(store.DurableExtents()) != 1 {
		t.Fatalf("durable extents = %d, want 1", len(store.DurableExtents()))
	}
	if err := store.Close(true); err != nil {
		t.Fatal(err)
	}
}
