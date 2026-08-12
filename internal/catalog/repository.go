package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	CatalogSnapshotName = "catalog.snapshot"
	CatalogJournalName  = "catalog.journal"
	CatalogLockName     = "catalog.lock"
	maxCatalogFileBytes = 64 << 20
)

var ErrCatalogLockUnavailable = errors.New("catalog mutation lock unavailable")

// Repository owns the durable local catalog. A repository has one writer or
// any number of read-only users; recovery remains independent of its health.
type Repository struct {
	root        string
	snapshot    string
	journal     string
	lockPath    string
	lockFile    *os.File
	journalFile *os.File
	store       Store
	mutable     bool
	closed      bool
}

// NewRepository creates a repository rooted at a caller-selected application
// data directory. It does not touch the filesystem until Open.
func NewRepository(root string) (*Repository, error) {
	if root == "" {
		return nil, errors.New("new catalog repository: root is required")
	}
	return &Repository{
		root:     root,
		snapshot: filepath.Join(root, CatalogSnapshotName),
		journal:  filepath.Join(root, CatalogJournalName),
		lockPath: filepath.Join(root, CatalogLockName),
	}, nil
}

// Open loads the snapshot and journal. A requested mutable open falls back to
// read-only mode when another process owns the lock.
func (r *Repository) Open(ctx context.Context, preferMutable bool) (OpenResult, error) {
	if r == nil || r.closed {
		return OpenResult{}, errors.New("open catalog repository: repository is closed")
	}
	if err := contextError(ctx); err != nil {
		return OpenResult{}, err
	}
	if err := os.MkdirAll(r.root, 0o755); err != nil {
		return OpenResult{}, fmt.Errorf("open catalog repository directory: %w", err)
	}
	if preferMutable {
		lock, err := acquireCatalogLock(r.lockPath)
		if err == nil {
			r.lockFile = lock
			r.mutable = true
		} else if !errors.Is(err, ErrCatalogLockUnavailable) {
			return OpenResult{}, fmt.Errorf("open catalog mutation lock: %w", err)
		}
	}
	if err := r.load(); err != nil {
		r.releaseLock()
		return OpenResult{}, err
	}
	if r.mutable {
		file, err := os.OpenFile(r.journal, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			r.releaseLock()
			return OpenResult{}, fmt.Errorf("open catalog journal: %w", err)
		}
		r.journalFile = file
		return OpenResult{Mode: OpenMutable, HistoryUpdatesAvailable: true}, nil
	}
	return OpenResult{Mode: OpenReadOnly, HistoryUpdatesAvailable: false}, nil
}

// Snapshot returns an immutable copy of the current catalog state.
func (r *Repository) Snapshot() Store {
	if r == nil {
		return Store{}
	}
	next := r.store
	next.Entries = append([]Entry(nil), r.store.Entries...)
	return next
}

// Lookup classifies prior work using the repository's in-memory index.

func (r *Repository) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	var firstErr error
	if r.journalFile != nil {
		if err := r.journalFile.Close(); err != nil {
			firstErr = err
		}
		r.journalFile = nil
	}
	if err := r.releaseLock(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
