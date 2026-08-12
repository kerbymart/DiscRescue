package recoverymap

import (
	"fmt"
	"io"
	"os"

	"discrescue/internal/mapfile"
)

func Create(path string, header mapfile.Header) (*Store, error) {
	if err := header.Validate(); err != nil {
		return nil, fmt.Errorf("create recovery map: %w", err)
	}
	header.CleanShutdown = false
	headerBytes, err := mapfile.MarshalHeader(header)
	if err != nil {
		return nil, fmt.Errorf("create recovery map header: %w", err)
	}
	checkpointBytes, err := mapfile.MarshalCheckpoint(mapfile.Checkpoint{})
	if err != nil {
		return nil, fmt.Errorf("create recovery map checkpoint: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create recovery map %s: %w", path, err)
	}
	store := &Store{path: path, file: file, header: header, nextSequence: 1}
	if err := store.writeAt(0, headerBytes); err != nil {
		return nil, store.abortCreate(err)
	}
	if err := store.writeAt(int64(len(headerBytes)), checkpointBytes); err != nil {
		return nil, store.abortCreate(err)
	}
	if err := file.Sync(); err != nil {
		return nil, store.abortCreate(fmt.Errorf("sync recovery map %s: %w", path, err))
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return nil, store.abortCreate(fmt.Errorf("seek recovery map journal %s: %w", path, err))
	}
	return store, nil
}

// Open validates and opens an existing recovery map for a writable session.
// Existing files are never removed or truncated by Open.
