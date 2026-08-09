package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryDurablyPersistsEventsAndReopens(t *testing.T) {
	root := t.TempDir()
	repository, err := NewRepository(root)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	opened, err := repository.Open(context.Background(), true)
	if err != nil || opened.Mode != OpenMutable {
		t.Fatalf("Open() = %+v, error = %v", opened, err)
	}
	entry := testCatalogEntry(9, 100, 100)
	if err := repository.AppendEvent(context.Background(), CatalogEvent{Type: EventMediaObserved, Entry: entry}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	if repository.Snapshot().LastSequence != 1 {
		t.Fatalf("expected durable sequence 1, got %d", repository.Snapshot().LastSequence)
	}
	if err := repository.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := NewRepository(root)
	if err != nil {
		t.Fatalf("NewRepository(reopen) error = %v", err)
	}
	opened, err = reopened.Open(context.Background(), false)
	if err != nil || opened.Mode != OpenReadOnly {
		t.Fatalf("read-only Open() = %+v, error = %v", opened, err)
	}
	if got := reopened.Snapshot(); got.LastSequence != 1 || len(got.Entries) != 1 || got.Entries[0].RecordID != entry.RecordID {
		t.Fatalf("reopened snapshot mismatch: %+v", got)
	}
	_ = reopened.Close()
}

func TestRepositoryFallsBackToReadOnlyWhenWriterIsHeld(t *testing.T) {
	root := t.TempDir()
	first, err := NewRepository(root)
	if err != nil {
		t.Fatalf("NewRepository(first) error = %v", err)
	}
	if _, err := first.Open(context.Background(), true); err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	second, err := NewRepository(root)
	if err != nil {
		t.Fatalf("NewRepository(second) error = %v", err)
	}
	opened, err := second.Open(context.Background(), true)
	if err != nil || opened.Mode != OpenReadOnly || opened.HistoryUpdatesAvailable {
		t.Fatalf("contended Open() = %+v, error = %v", opened, err)
	}
	_ = second.Close()
	_ = first.Close()
	if _, err := os.Stat(filepath.Join(root, CatalogLockName)); !os.IsNotExist(err) {
		t.Fatalf("expected writer lock to be released, stat error = %v", err)
	}
}
