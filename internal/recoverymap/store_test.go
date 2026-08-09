package recoverymap

import (
	"path/filepath"
	"testing"

	"discrescue/internal/mapfile"
)

func testHeader() mapfile.Header {
	return mapfile.Header{
		LogicalSectorSize:        2048,
		ExpectedSectorCount:      128,
		OutputFormat:             1,
		IdentityAlgorithmVersion: 7,
		CreationUnixNano:         1234,
		CleanShutdown:            true,
	}
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
	if header.IdentityAlgorithmVersion != 7 || header.CreationUnixNano != 1234 || header.OutputFormat != 1 {
		t.Fatalf("metadata changed during resume: %+v", header)
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
