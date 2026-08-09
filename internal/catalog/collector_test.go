package catalog

import (
	"context"
	"testing"
	"time"
)

func TestBuildSamplePlanIsBoundedAndDistributed(t *testing.T) {
	plan, err := BuildSamplePlan(1000)
	if err != nil {
		t.Fatalf("BuildSamplePlan() error = %v", err)
	}
	if len(plan) != 8 || plan[0].LBA != 0 || plan[1].LBA != 124 || plan[6].LBA != 749 || plan[7].LBA != 999 {
		t.Fatalf("unexpected sample plan: %+v", plan)
	}

	small, err := BuildSamplePlan(2)
	if err != nil {
		t.Fatalf("BuildSamplePlan(small) error = %v", err)
	}
	if len(small) != 2 || small[0].Slot != 0 || small[1].Slot != 7 {
		t.Fatalf("expected duplicate LBAs to be removed deterministically: %+v", small)
	}
}

func TestFingerprintCollectorProducesQuickIDAndExplicitUnavailableSamples(t *testing.T) {
	reader := fakeSectorReader{sectorSize: 4, unavailable: map[int64]bool{999: true}}
	base := ContentIdentity{Version: 1, LogicalBlockSize: 4, SectorCount: 1000}
	identity, stats, err := (FingerprintCollector{TotalBudget: time.Second, ReadDeadline: time.Second}).Collect(context.Background(), base, reader)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if stats.AttemptedSamples != 8 || stats.AvailableSamples != 7 || stats.UnavailableSamples != 1 || stats.BytesRead != 28 {
		t.Fatalf("unexpected collection stats: %+v", stats)
	}
	if identity.QuickID != "" {
		t.Fatalf("expected QuickID to be absent with an unavailable mandatory sample, got %q", identity.QuickID)
	}
	last := identity.Samples[len(identity.Samples)-1]
	if last.Available || last.Error != SampleErrorRead || last.SHA256 != ([32]byte{}) {
		t.Fatalf("expected explicit unavailable final sample, got %+v", last)
	}
}

func TestFingerprintCollectorSameContentProducesSameQuickID(t *testing.T) {
	reader := fakeSectorReader{sectorSize: 4}
	base := ContentIdentity{Version: 1, LogicalBlockSize: 4, SectorCount: 1000}
	left, _, err := (FingerprintCollector{}).Collect(context.Background(), base, reader)
	if err != nil {
		t.Fatalf("left Collect() error = %v", err)
	}
	right, _, err := (FingerprintCollector{}).Collect(context.Background(), base, reader)
	if err != nil {
		t.Fatalf("right Collect() error = %v", err)
	}
	if left.QuickID == "" || left.QuickID != right.QuickID {
		t.Fatalf("expected deterministic QuickID, left=%q right=%q", left.QuickID, right.QuickID)
	}
}

type fakeSectorReader struct {
	sectorSize  uint32
	unavailable map[int64]bool
}

func (r fakeSectorReader) ReadSector(ctx context.Context, lba int64, sectorSize uint32) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if r.unavailable[lba] {
		return nil, context.Canceled
	}
	data := make([]byte, sectorSize)
	for i := range data {
		data[i] = byte((lba + int64(i)) % 251)
	}
	return data, nil
}
