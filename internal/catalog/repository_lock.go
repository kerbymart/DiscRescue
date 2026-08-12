package catalog

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

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
