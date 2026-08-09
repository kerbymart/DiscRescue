package recoverymap

import (
	"errors"
	"os"
)

// StartupTransaction removes only artifacts created by a new recovery attempt
// until the attempt has reached its durable-running point.
type StartupTransaction struct {
	paths     []string
	committed bool
}

// TrackCreated records an artifact created by the current startup attempt.
func (t *StartupTransaction) TrackCreated(path string) {
	if t == nil || path == "" || t.committed {
		return
	}
	t.paths = append(t.paths, path)
}

// Commit makes tracked artifacts durable and prevents rollback removal.
func (t *StartupTransaction) Commit() {
	if t != nil {
		t.committed = true
	}
}

// Rollback removes tracked artifacts when startup did not reach commit.
func (t *StartupTransaction) Rollback() error {
	if t == nil || t.committed {
		return nil
	}
	var joined error
	for i := len(t.paths) - 1; i >= 0; i-- {
		if err := os.Remove(t.paths[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}
