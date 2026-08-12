package recovery

import (
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"discrescue/internal/mapfile"
	"discrescue/internal/recoverymap"
)

type persistenceTestWriter struct {
	data      []byte
	syncCalls int
}

func (w *persistenceTestWriter) WriteAt(data []byte, offset int64) (int, error) {
	if offset < 0 || offset > int64(len(w.data)) || int64(len(data)) > int64(len(w.data))-offset {
		return 0, io.ErrShortWrite
	}
	copy(w.data[offset:], data)
	return len(data), nil
}

func (w *persistenceTestWriter) Sync() error {
	w.syncCalls++
	return nil
}

func TestRecoveryPersistenceOrdersImageSyncBeforeDurableMapProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.drmap")
	store, err := recoverymap.Create(path, mapfile.Header{LogicalSectorSize: 1, ExpectedSectorCount: 4})
	if err != nil {
		t.Fatal(err)
	}
	writer := &persistenceTestWriter{data: make([]byte, 4)}
	persistence := NewPersistence(writer, store)
	extent := mapfile.Extent{StartLBA: 0, Sectors: 1, State: mapfile.SectorStateReadUnverified, Confidence: mapfile.ConfidenceSingleRead}
	if err := persistence.PersistRecovered([]byte{0xA5}, 0, extent); err != nil {
		t.Fatal(err)
	}
	if writer.syncCalls != 0 || len(store.DurableExtents()) != 0 {
		t.Fatalf("staged write became durable too early: syncs=%d durable=%+v", writer.syncCalls, store.DurableExtents())
	}
	if err := persistence.ForceCheckpoint(CheckpointReasonStop); err != nil {
		t.Fatal(err)
	}
	if writer.syncCalls != 1 || len(store.DurableExtents()) != 1 {
		t.Fatalf("checkpoint did not commit image before map: syncs=%d durable=%+v", writer.syncCalls, store.DurableExtents())
	}
	if stats := persistence.Stats(); stats.Count != 1 || stats.LastReason != CheckpointReasonStop {
		t.Fatalf("unexpected checkpoint stats: %+v", stats)
	}
	if err := store.Close(true); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryPersistenceIntervalTriggersAfterCompletedOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.drmap")
	store, err := recoverymap.Create(path, mapfile.Header{LogicalSectorSize: 1, ExpectedSectorCount: 4})
	if err != nil {
		t.Fatal(err)
	}
	writer := &persistenceTestWriter{data: make([]byte, 4)}
	persistence := NewPersistence(writer, store)
	now := time.Unix(100, 0)
	persistence.now = func() time.Time { return now }
	persistence.lastCheckpoint = now
	if err := persistence.PersistRecovered([]byte{1}, 0, mapfile.Extent{StartLBA: 0, Sectors: 1, State: mapfile.SectorStateReadUnverified, Confidence: mapfile.ConfidenceSingleRead}); err != nil {
		t.Fatal(err)
	}
	if writer.syncCalls != 0 {
		t.Fatalf("unexpected early interval checkpoint: %d", writer.syncCalls)
	}
	now = now.Add(MaxCheckpointInterval)
	if err := persistence.PersistRecovered([]byte{2}, 1, mapfile.Extent{StartLBA: 1, Sectors: 1, State: mapfile.SectorStateReadUnverified, Confidence: mapfile.ConfidenceSingleRead}); err != nil {
		t.Fatal(err)
	}
	if writer.syncCalls != 1 || persistence.Stats().LastReason != CheckpointReasonInterval {
		t.Fatalf("interval did not trigger checkpoint: syncs=%d stats=%+v", writer.syncCalls, persistence.Stats())
	}
	if err := store.Close(true); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkRecoveryPersistenceHealthy(b *testing.B) {
	const sectors = 64
	data := make([]byte, 2048)
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		path := filepath.Join(b.TempDir(), fmt.Sprintf("capture-%d.drmap", iteration))
		store, err := recoverymap.Create(path, mapfile.Header{LogicalSectorSize: 2048, ExpectedSectorCount: sectors})
		if err != nil {
			b.Fatal(err)
		}
		writer := &persistenceTestWriter{data: make([]byte, sectors*2048)}
		persistence := NewPersistence(writer, store)
		for lba := uint64(0); lba < sectors; lba++ {
			if err := persistence.PersistRecovered(data, int64(lba*2048), mapfile.Extent{StartLBA: lba, Sectors: 1, State: mapfile.SectorStateReadUnverified, Confidence: mapfile.ConfidenceSingleRead}); err != nil {
				b.Fatal(err)
			}
		}
		if err := persistence.ForceCheckpoint(CheckpointReasonRecoveryReturn); err != nil {
			b.Fatal(err)
		}
		if writer.syncCalls >= sectors {
			b.Fatalf("batched checkpoint performed %d image syncs for %d reads", writer.syncCalls, sectors)
		}
		if err := store.Close(true); err != nil {
			b.Fatal(err)
		}
	}
}
