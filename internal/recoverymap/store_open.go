package recoverymap

import (
	"fmt"
	"io"
	"os"

	"discrescue/internal/mapfile"
)

func Open(path string, geometry Geometry) (*Store, error) {
	if geometry.LogicalSectorSize == 0 || geometry.ExpectedSectorCount == 0 {
		return nil, fmt.Errorf("open recovery map: media geometry is required")
	}
	header, replayed, err := readState(path, geometry)
	if err != nil {
		return nil, fmt.Errorf("open recovery map %s: %w", path, err)
	}
	if replayed.LastSequence == ^uint64(0) {
		return nil, fmt.Errorf("open recovery map %s: sequence exhausted", path)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open recovery map %s: %w", path, err)
	}
	store := &Store{
		path:           path,
		file:           file,
		header:         header,
		extents:        append([]mapfile.Extent(nil), replayed.Extents...),
		durableExtents: append([]mapfile.Extent(nil), replayed.Extents...),
		nextSequence:   replayed.LastSequence + 1,
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return nil, store.abort(fmt.Errorf("seek recovery map journal %s: %w", path, err))
	}
	// A writable session is unclean until finalization has completed.
	store.header.CleanShutdown = false
	headerBytes, err := mapfile.MarshalHeader(store.header)
	if err != nil {
		return nil, store.abort(fmt.Errorf("mark recovery map active: %w", err))
	}
	if err := store.writeAt(0, headerBytes); err != nil {
		return nil, store.abort(fmt.Errorf("mark recovery map active: %w", err))
	}
	if err := file.Sync(); err != nil {
		return nil, store.abort(fmt.Errorf("sync active recovery map %s: %w", path, err))
	}
	return store, nil
}

// Inspect validates an existing map without opening it for a writable
// session or changing its clean-shutdown header.
