package recovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

type deadlineRecoveryReader struct {
	data  []byte
	mu    sync.Mutex
	calls []int64
}

func (r *deadlineRecoveryReader) ReadAt(p []byte, off int64) (int, error) {
	return r.ReadAtContext(context.Background(), p, off)
}

func (r *deadlineRecoveryReader) ReadAtContext(ctx context.Context, p []byte, off int64) (int, error) {
	r.mu.Lock()
	r.calls = append(r.calls, off)
	r.mu.Unlock()
	if off == 0 {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	copy(p, r.data[int(off):int(off)+len(p)])
	return len(p), nil
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

func TestPassBasedRecoveryTimeoutDefersRangeAndContinuesForward(t *testing.T) {
	reader := &deadlineRecoveryReader{data: newRecoveryTestData(128)}
	writer := &memoryRecoveryWriter{data: make([]byte, 128)}
	store := &memoryRecoveryStore{}
	policy, err := PolicyForMethod(RecoveryMethodFast)
	if err != nil {
		t.Fatal(err)
	}
	policy.ReadDeadlines = ReadDeadlinePolicy{
		HealthySoft: time.Millisecond,
		HealthyHard: 5 * time.Millisecond,
		DamagedSoft: time.Millisecond,
		DamagedHard: 5 * time.Millisecond,
	}
	var sawTimeout bool
	err = runPassBasedRecoveryWithPolicy(context.Background(), reader, writer, 1, 128, store, policy, func(progress recoveryPassProgress) {
		for _, issue := range progress.LastIssue {
			if strings.Contains(issue, "deadline exceeded") {
				sawTimeout = true
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawTimeout {
		t.Fatal("expected the blocked fast-pass read to be reported as a timeout")
	}
	if len(reader.calls) < 2 || reader.calls[1] != 64 {
		t.Fatalf("expected recovery to continue to LBA 64 after timeout, calls=%v", reader.calls)
	}
	if extent, _, ok := mapfile.LookupExtent(store.Extents(), 64); !ok || !RecoveryStateHasData(extent.State) {
		t.Fatalf("expected later readable range to recover, got %+v", extent)
	}
}

func TestPassBasedRecoveryPolicyControlsPassSequence(t *testing.T) {
	for _, test := range []struct {
		name          string
		method        RecoveryMethod
		wantTargeted  bool
		wantFinalPass string
	}{
		{name: "fast", method: RecoveryMethodFast, wantTargeted: false, wantFinalPass: "Deferred work remains"},
		{name: "balanced", method: RecoveryMethodBalanced, wantTargeted: true, wantFinalPass: "Complete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			policy, err := PolicyForMethod(test.method)
			if err != nil {
				t.Fatal(err)
			}
			reader := &scriptedRecoveryReader{failAll: true, data: newRecoveryTestData(8)}
			writer := &memoryRecoveryWriter{data: make([]byte, 8)}
			store := &memoryRecoveryStore{}
			var passes []string
			err = runPassBasedRecoveryWithPolicy(context.Background(), reader, writer, 1, 8, store, policy, func(progress recoveryPassProgress) {
				passes = append(passes, progress.Pass)
			})
			if err != nil {
				t.Fatal(err)
			}
			foundTargeted := false
			for _, pass := range passes {
				if pass == "Targeted retry" {
					foundTargeted = true
				}
			}
			if foundTargeted != test.wantTargeted {
				t.Fatalf("targeted pass=%v, want %v; passes=%v", foundTargeted, test.wantTargeted, passes)
			}
			if passes[len(passes)-1] != test.wantFinalPass {
				t.Fatalf("final pass=%q, want %q; passes=%v", passes[len(passes)-1], test.wantFinalPass, passes)
			}
		})
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
		if !ok || !RecoveryStateHasData(extent.State) {
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
	if !ok || !RecoveryStateHasData(extent.State) {
		t.Fatalf("expected sector 70 to be recovered, got %+v", extent)
	}
	if extent.Attempts > maxTargetedAttempts {
		t.Fatalf("retry attempts exceeded documented cap: %+v", extent)
	}
}

func TestPassBasedRecoveryCompletesCoverageAcrossLargeDamagedBand(t *testing.T) {
	const sectors = 704
	permanentBad := make(map[int]bool, 576)
	for sector := 0; sector < 576; sector++ {
		permanentBad[sector] = true
	}
	reader := &scriptedRecoveryReader{
		data:         newRecoveryTestData(sectors),
		permanentBad: permanentBad,
	}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}

	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, nil); err != nil {
		t.Fatalf("run recovery: %v", err)
	}
	scanned, recovered, deferred, unreadable := SummarizeRecoveryExtentStates(store.Extents())
	if scanned != sectors || recovered != 128 || deferred != 0 || unreadable != 576 {
		t.Fatalf("unexpected final coverage: scanned=%d recovered=%d deferred=%d unreadable=%d extents=%+v", scanned, recovered, deferred, unreadable, store.Extents())
	}
	if recovered+unreadable != sectors {
		t.Fatalf("finalized coverage invariant failed: recovered=%d unreadable=%d capacity=%d", recovered, unreadable, sectors)
	}
	if !recoveryReaderCalledRange(reader.calls, 576, 64) {
		t.Fatalf("fast pass never reached readable sectors after the damaged band: %+v", reader.calls)
	}
}

func TestPassBasedRecoveryCompletesEntireUnreadableMedium(t *testing.T) {
	const sectors = 640
	reader := &scriptedRecoveryReader{data: newRecoveryTestData(sectors), failAll: true}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}

	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, nil); err != nil {
		t.Fatalf("run recovery: %v", err)
	}
	scanned, recovered, deferred, unreadable := SummarizeRecoveryExtentStates(store.Extents())
	if scanned != sectors || recovered != 0 || deferred != 0 || unreadable != sectors {
		t.Fatalf("unexpected final coverage: scanned=%d recovered=%d deferred=%d unreadable=%d extents=%+v", scanned, recovered, deferred, unreadable, store.Extents())
	}
}

func TestPassBasedRecoveryReportsNativeReadFailureWithoutAbortingCoverage(t *testing.T) {
	const sectors = 128
	reader := &scriptedRecoveryReader{
		data:         newRecoveryTestData(sectors),
		permanentBad: map[int]bool{0: true},
	}
	writer := &memoryRecoveryWriter{data: make([]byte, sectors)}
	store := &memoryRecoveryStore{}
	var issues []string

	if err := runPassBasedRecovery(context.Background(), reader, writer, 1, sectors, store, func(progress recoveryPassProgress) {
		issues = append(issues, progress.LastIssue...)
	}); err != nil {
		t.Fatalf("run recovery: %v", err)
	}
	if !containsRecoveryIssue(issues, "unreadable sector") {
		t.Fatalf("expected native read error in progress diagnostics, got %v", issues)
	}
}

type permissionDeniedRecoveryReader struct{}

func (permissionDeniedRecoveryReader) ReadAt([]byte, int64) (int, error) {
	return 0, os.ErrPermission
}

func TestPassBasedRecoveryFailsWhenSourceAccessIsRevoked(t *testing.T) {
	writer := &memoryRecoveryWriter{data: make([]byte, 64)}
	store := &memoryRecoveryStore{}
	err := runPassBasedRecovery(context.Background(), permissionDeniedRecoveryReader{}, writer, 1, 64, store, nil)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission failure to remain fatal, got %v", err)
	}
	if len(store.Extents()) != 0 {
		t.Fatalf("fatal source failure must not claim coverage, got %+v", store.Extents())
	}
}

func TestRetryPolicyAddsOneBoundedBudgetAboveDurableAttempts(t *testing.T) {
	policy, err := PolicyForMethod(RecoveryMethodBalanced)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	retry := RetryPolicyWithCurrentAttempts(policy, []mapfile.Extent{
		{StartLBA: 0, Sectors: 1, State: mapfile.SectorStateMissing, Attempts: 6},
		{StartLBA: 1, Sectors: 1, State: mapfile.SectorStateReadUnverified, Attempts: 99},
	})
	if retry.Trim.AttemptsLimit != 8 {
		t.Fatalf("trim retry limit = %d, want 8", retry.Trim.AttemptsLimit)
	}
	if retry.Adaptive[0].AttemptsLimit != 9 || retry.Adaptive[1].AttemptsLimit != 10 {
		t.Fatalf("adaptive retry limits = %+v, want [9 10]", retry.Adaptive)
	}
	if retry.Targeted.AttemptsLimit != 12 {
		t.Fatalf("targeted retry limit = %d, want 12", retry.Targeted.AttemptsLimit)
	}
}

func TestRetryPolicyRevisitsPreviouslyExhaustedMissingExtent(t *testing.T) {
	policy, err := PolicyForMethod(RecoveryMethodBalanced)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	store := &memoryRecoveryStore{extents: []mapfile.Extent{
		{StartLBA: 0, Sectors: 1, State: mapfile.SectorStateMissing, Confidence: mapfile.ConfidenceNone, Attempts: 6},
	}}
	reader := &scriptedRecoveryReader{data: []byte{0x5A}}
	writer := &memoryRecoveryWriter{data: make([]byte, 1)}

	if err := runPassBasedRecoveryWithPolicy(context.Background(), reader, writer, 1, 1, store, RetryPolicyWithCurrentAttempts(policy, store.Extents()), nil); err != nil {
		t.Fatalf("retry recovery: %v", err)
	}
	_, recovered, deferred, unreadable := SummarizeRecoveryExtentStates(store.Extents())
	if recovered != 1 || deferred != 0 || unreadable != 0 {
		t.Fatalf("retry did not recover exhausted extent: recovered=%d deferred=%d unreadable=%d extents=%+v", recovered, deferred, unreadable, store.Extents())
	}
}

func containsRecoveryIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}

func recoveryReaderCalledRange(calls []recoveryReadCall, start, sectors int) bool {
	for _, call := range calls {
		if call.start == start && call.sectors == sectors {
			return true
		}
	}
	return false
}
