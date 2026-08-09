package catalog

import (
	"context"
	"testing"
	"time"
)

func TestServiceRecordsObservedContentAndJobState(t *testing.T) {
	repository, err := NewRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	if _, err := repository.Open(context.Background(), true); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	service := NewService(repository)
	service.now = func() time.Time { return time.Unix(0, 100) }
	identity := identityWithSamplesAndQuick("layout-service", []SectorFingerprint{availableSample(1, 0, "one")}, "")
	capture := CaptureIdentity{CaptureID: "capture-1", Device: DeviceIdentity{Vendor: "test"}, StartedAt: time.Unix(0, 50)}
	recordID, status := service.RecordObserved(context.Background(), IdentityObservation{Identity: identity}, capture)
	if status.State != CatalogWriteRecorded {
		t.Fatalf("RecordObserved() status = %+v", status)
	}
	jobID := RecordID{8}
	status = service.RecordJobStarted(context.Background(), recordID, JobMetadata{JobID: jobID, ImagePath: "/tmp/disc.iso"})
	if status.State != CatalogWriteRecorded {
		t.Fatalf("RecordJobStarted() status = %+v", status)
	}
	if got := repository.Snapshot().Entries[0]; got.State != ProcessingInProgress || got.PreferredJobID != jobID {
		t.Fatalf("unexpected catalog entry after job start: %+v", got)
	}
	_ = repository.Close()
}
