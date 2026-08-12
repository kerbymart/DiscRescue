package catalog

import (
	"context"
	"errors"
	"fmt"
)

func (r *Repository) AppendEvent(ctx context.Context, event CatalogEvent) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if r == nil || r.closed || !r.mutable || r.journalFile == nil {
		return errors.New("append catalog event: repository is not mutable")
	}
	sequence := r.store.LastSequence + 1
	record, err := EncodeCatalogEvent(event, sequence)
	if err != nil {
		return fmt.Errorf("append catalog event: %w", err)
	}
	encoded, err := MarshalJournalRecord(record)
	if err != nil {
		return fmt.Errorf("append catalog event: marshal: %w", err)
	}
	if _, err := r.journalFile.Write(encoded); err != nil {
		return fmt.Errorf("append catalog journal: %w", err)
	}
	if err := r.journalFile.Sync(); err != nil {
		return fmt.Errorf("sync catalog journal: %w", err)
	}
	next, err := ApplyEvent(r.store, event, sequence)
	if err != nil {
		return fmt.Errorf("apply catalog event after durable append: %w", err)
	}
	r.store = next
	return nil
}

// Close closes the journal and releases the writer lock. A stale lock is not
// silently removed; callers must verify the owning process is gone before
// deleting it and reopening, preventing concurrent catalog writers.
