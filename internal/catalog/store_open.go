package catalog

import "fmt"

type OpenMode string

const (
	OpenReadOnly OpenMode = "read_only"
	OpenMutable  OpenMode = "mutable"
)

type OpenResult struct {
	Mode                    OpenMode
	HistoryUpdatesAvailable bool
}

func (s Store) Open(preferMutable bool) (OpenResult, error) {
	if preferMutable {
		if s.MutationLockHeld {
			return OpenResult{}, fmt.Errorf("open store: mutation lock is already held")
		}
		return OpenResult{Mode: OpenMutable, HistoryUpdatesAvailable: true}, nil
	}
	return OpenResult{Mode: OpenReadOnly, HistoryUpdatesAvailable: !s.MutationLockHeld}, nil
}
