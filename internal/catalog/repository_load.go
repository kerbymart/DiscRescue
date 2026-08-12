package catalog

import (
	"errors"
	"fmt"
	"os"
)

func (r *Repository) load() error {
	r.store = Store{}
	snapshot := Snapshot{}
	if data, err := readBoundedFile(r.snapshot); err == nil {
		snapshot, err = UnmarshalSnapshot(data)
		if err != nil {
			return fmt.Errorf("load catalog snapshot: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read catalog snapshot: %w", err)
	}
	journaling, err := readBoundedFile(r.journal)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read catalog journal: %w", err)
	}
	r.store, err = ReplayJournal(snapshot, journaling)
	if err != nil {
		return fmt.Errorf("replay catalog journal: %w", err)
	}
	return nil
}
