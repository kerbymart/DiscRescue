package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"discrescue/internal/mapfile"
)

type memoryRecoveryStore struct {
	extents []mapfile.Extent
}

func (s *memoryRecoveryStore) Extents() []mapfile.Extent {
	return append([]mapfile.Extent(nil), s.extents...)
}

func (s *memoryRecoveryStore) ApplyExtent(extent mapfile.Extent) error {
	next, err := mapfile.ApplyExtent(s.extents, extent)
	if err != nil {
		return err
	}
	s.extents = next
	return nil
}

type stagedMemoryRecoveryStore struct {
	extents        []mapfile.Extent
	durableExtents []mapfile.Extent
	pending        bool
}

func (s *stagedMemoryRecoveryStore) Extents() []mapfile.Extent {
	return append([]mapfile.Extent(nil), s.extents...)
}

func (s *stagedMemoryRecoveryStore) DurableExtents() []mapfile.Extent {
	return append([]mapfile.Extent(nil), s.durableExtents...)
}

func (s *stagedMemoryRecoveryStore) ApplyExtent(extent mapfile.Extent) error {
	if err := s.Flush(); err != nil {
		return err
	}
	next, err := mapfile.ApplyExtent(s.extents, extent)
	if err != nil {
		return err
	}
	s.extents = next
	s.durableExtents = append([]mapfile.Extent(nil), next...)
	return nil
}

func (s *stagedMemoryRecoveryStore) StageExtent(extent mapfile.Extent) error {
	next, err := mapfile.ApplyExtent(s.extents, extent)
	if err != nil {
		return err
	}
	s.extents = next
	s.pending = true
	return nil
}

func (s *stagedMemoryRecoveryStore) Flush() error {
	if s.pending {
		s.durableExtents = append([]mapfile.Extent(nil), s.extents...)
		s.pending = false
	}
	return nil
}

func (s *stagedMemoryRecoveryStore) PendingBytes() uint64 {
	if s.pending {
		return 1
	}
	return 0
}

type memoryRecoveryWriter struct {
	data []byte
}

func (w *memoryRecoveryWriter) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	end := int(off) + len(p)
	if end > len(w.data) {
		return 0, io.ErrShortWrite
	}
	copy(w.data[int(off):end], p)
	return len(p), nil
}

func (w *memoryRecoveryWriter) Sync() error { return nil }

type recoveryReadCall struct {
	start   int
	sectors int
}

type scriptedRecoveryReader struct {
	data             []byte
	permanentBad     map[int]bool
	transientFailure map[int]int
	failAll          bool
	calls            []recoveryReadCall
}

func (r *scriptedRecoveryReader) ReadAt(p []byte, off int64) (int, error) {
	start := int(off)
	r.calls = append(r.calls, recoveryReadCall{start: start, sectors: len(p)})
	if r.failAll {
		return 0, errors.New("media unavailable")
	}
	for sector := start; sector < start+len(p); sector++ {
		if r.permanentBad[sector] {
			return 0, errors.New("unreadable sector")
		}
		if remaining := r.transientFailure[sector]; remaining > 0 {
			r.transientFailure[sector] = remaining - 1
			return 0, errors.New("transient read failure")
		}
	}
	copy(p, r.data[start:start+len(p)])
	return len(p), nil
}

func newRecoveryTestData(sectors int) []byte {
	data := make([]byte, sectors)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func TestPassBasedRecoveryCompletesFastCoverageBeforeTargetedRetry(t *testing.T) {
	const sectors = 192
	reader := &scriptedRecoveryReader{
		data:         newRecoveryTestData(sectors),
		permanentBad: map[int]bool{70: true},
	}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var coverageReached bool
	err := runPassBasedRecovery(ctx, reader, writer, 1, sectors, store, func(progress recoveryPassProgress) {
		if progress.Pass == "Fast acquisition" && progress.ScannedSectors == sectors {
			coverageReached = true
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation after fast coverage, got %v", err)
	}
	if !coverageReached {
		t.Fatal("expected the fast pass to scan the whole image before retry work")
	}
	if len(reader.calls) != 3 {
		t.Fatalf("expected exactly three sequential fast reads before cancellation, got %d: %+v", len(reader.calls), reader.calls)
	}
	for i, call := range reader.calls {
		if call.sectors != 64 {
			t.Fatalf("fast read %d used %d sectors, want 64", i, call.sectors)
		}
	}
	_, recovered, unresolved := summarizeRecoveryExtents(store.Extents())
	if recovered != 128 || unresolved != 64 {
		t.Fatalf("unexpected fast-pass state: recovered=%d unresolved=%d extents=%+v", recovered, unresolved, store.Extents())
	}
}

func TestPassBasedRecoveryPublishesOnlyDurableBatchedProgress(t *testing.T) {
	const sectors = 64
	reader := &scriptedRecoveryReader{data: newRecoveryTestData(sectors)}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &stagedMemoryRecoveryStore{}
	var progress []recoveryPassProgress
	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, func(update recoveryPassProgress) {
		progress = append(progress, update)
	}); err != nil {
		t.Fatalf("run recovery: %v", err)
	}
	if len(progress) == 0 || progress[len(progress)-1].Pass != "Complete" {
		t.Fatalf("unexpected progress updates: %+v", progress)
	}
	for _, update := range progress {
		if update.Pass != "Complete" && update.RecoveredSectors != 0 {
			t.Fatalf("published staged recovery as durable: %+v", update)
		}
	}
	if got := progress[len(progress)-1].RecoveredSectors; got != sectors {
		t.Fatalf("final durable progress = %d, want %d", got, sectors)
	}
}

func TestPassBasedRecoveryNarrowsDamageToBadSector(t *testing.T) {
	const sectors = 192
	reader := &scriptedRecoveryReader{
		data:         newRecoveryTestData(sectors),
		permanentBad: map[int]bool{70: true},
	}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}

	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, nil); err != nil {
		t.Fatalf("run recovery: %v", err)
	}
	_, recovered, unresolved := summarizeRecoveryExtents(store.Extents())
	if recovered != sectors-1 || unresolved != 1 {
		t.Fatalf("expected only one unresolved sector, got recovered=%d unresolved=%d extents=%+v", recovered, unresolved, store.Extents())
	}
	bad, _, ok := mapfile.LookupExtent(store.Extents(), 70)
	if !ok || bad.State != mapfile.SectorStateMissing || bad.Sectors != 1 {
		t.Fatalf("expected sector 70 to be the only missing sector, got %+v", bad)
	}
	for _, lba := range []uint64{69, 71} {
		extent, _, ok := mapfile.LookupExtent(store.Extents(), lba)
		if !ok || !recoveryStateHasData(extent.State) {
			t.Fatalf("expected neighboring sector %d to be recovered, got %+v", lba, extent)
		}
	}
}

func TestPassBasedRecoveryHandlesIntermittentRead(t *testing.T) {
	const sectors = 192
	reader := &scriptedRecoveryReader{
		data:             newRecoveryTestData(sectors),
		permanentBad:     map[int]bool{},
		transientFailure: map[int]int{70: 2},
	}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}

	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, nil); err != nil {
		t.Fatalf("run recovery: %v", err)
	}
	_, recovered, unresolved := summarizeRecoveryExtents(store.Extents())
	if recovered != sectors || unresolved != 0 {
		t.Fatalf("expected intermittent sector to recover, got recovered=%d unresolved=%d extents=%+v", recovered, unresolved, store.Extents())
	}
	extent, _, ok := mapfile.LookupExtent(store.Extents(), 70)
	if !ok || !recoveryStateHasData(extent.State) {
		t.Fatalf("expected sector 70 to be recovered, got %+v", extent)
	}
	if extent.Attempts > maxTargetedAttempts {
		t.Fatalf("retry attempts exceeded documented cap: %+v", extent)
	}
}

func TestPassBasedRecoveryPreservesDeferredStateAcrossRestart(t *testing.T) {
	const sectors = 640
	reader := &scriptedRecoveryReader{
		data:         newRecoveryTestData(sectors),
		permanentBad: map[int]bool{},
		failAll:      true,
	}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}

	err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, nil)
	if !errors.Is(err, errRecoveryConsecutiveFailures) {
		t.Fatalf("expected systemic-read guard to stop the run, got %v", err)
	}
	scanned, recovered, unresolved := summarizeRecoveryExtents(store.Extents())
	if scanned != 512 || recovered != 0 || unresolved != 512 {
		t.Fatalf("expected eight failed fast blocks to be durable, got scanned=%d recovered=%d unresolved=%d extents=%+v", scanned, recovered, unresolved, store.Extents())
	}

	reader.failAll = false
	reader.calls = nil
	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, nil); err != nil {
		t.Fatalf("resume recovery: %v", err)
	}
	_, recovered, unresolved = summarizeRecoveryExtents(store.Extents())
	if recovered != sectors || unresolved != 0 {
		t.Fatalf("expected resumed recovery to finish all sectors, got recovered=%d unresolved=%d", recovered, unresolved)
	}
}
