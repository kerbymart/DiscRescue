package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
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
func (r *Repository) Lookup(ctx context.Context, identity ContentIdentity, budget LookupBudget) (LookupResult, error) {
	if err := contextError(ctx); err != nil {
		return LookupResult{}, err
	}
	if r == nil || r.closed {
		return LookupResult{}, errors.New("lookup catalog: repository is closed")
	}
	return Lookup(r.store.Entries, identity, budget)
}

// LookupPriorProcessing classifies matching catalog work and permits
// automatic resume only for a strong resumable candidate whose current image
// and map files both exist.
func (r *Repository) LookupPriorProcessing(ctx context.Context, observation IdentityObservation, budget LookupBudget) (PriorProcessingResult, error) {
	lookup, err := r.Lookup(ctx, observation.Identity, budget)
	if err != nil {
		return PriorProcessingResult{}, err
	}
	for i := range lookup.Candidates {
		lookup.Candidates[i] = refreshCandidateFiles(lookup.Candidates[i])
	}
	copyResult, err := BuildPriorProcessingCopy(lookup)
	if err != nil {
		return PriorProcessingResult{}, err
	}
	result := PriorProcessingResult{Match: lookup.Match, Candidates: lookup.Candidates}
	switch lookup.Match {
	case MatchStrong:
		if copyResult.State == PriorProcessingIncomplete {
			result.Kind = PriorStrongResume
			result.AutoResumeAllowed = true
		} else if copyResult.State == PriorProcessingUnavailable {
			result.Kind = PriorUnavailable
		} else {
			result.Kind = PriorStrongDone
		}
	case MatchProbable:
		result.Kind = PriorProbable
	case MatchIndeterminate:
		result.Kind = PriorIndeterminate
	case MatchConflict:
		result.Kind = PriorConflict
	default:
		result.Kind = PriorNone
	}
	result.Detail = copyResult.Detail
	return result, nil
}

// AppendEvent durably appends and syncs one event before applying it in memory.
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

func acquireCatalogLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrCatalogLockUnavailable
		}
		return nil, err
	}
	if _, err := fmt.Fprintf(file, "pid=%s\nstarted=%s\n", strconv.Itoa(os.Getpid()), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return file, nil
}

func (r *Repository) releaseLock() error {
	if r.lockFile == nil {
		return nil
	}
	closeErr := r.lockFile.Close()
	r.lockFile = nil
	removeErr := os.Remove(r.lockPath)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	return closeErr
}

func readBoundedFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxCatalogFileBytes {
		return nil, fmt.Errorf("catalog file %s exceeds %d bytes", path, maxCatalogFileBytes)
	}
	return os.ReadFile(path)
}

func refreshCandidateFiles(entry Entry) Entry {
	refreshed := entry
	refreshed.JobReferences = append([]JobReference(nil), entry.JobReferences...)
	for i := range refreshed.JobReferences {
		imageInfo, imageErr := os.Stat(refreshed.JobReferences[i].Path)
		mapPath := replaceCatalogMapExtension(refreshed.JobReferences[i].Path)
		_, mapErr := os.Stat(mapPath)
		refreshed.JobReferences[i].FilesPresent = imageErr == nil && imageInfo.Mode().IsRegular() && mapErr == nil
	}
	return refreshed
}

func replaceCatalogMapExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == ".iso" {
		return path[:len(path)-len(ext)] + ".drmap"
	}
	return path + ".drmap"
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
