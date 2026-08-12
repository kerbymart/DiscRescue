package recoverymap

import (
	"errors"
	"fmt"
	"io"

	"discrescue/internal/mapfile"
)

func (s *Store) Flush() error {
	if s == nil || s.file == nil || s.closed {
		return errors.New("flush recovery map journal: store is closed")
	}
	if len(s.pendingRecords) == 0 {
		return nil
	}
	if _, err := s.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek recovery map journal %s: %w", s.path, err)
	}
	for _, record := range s.pendingRecords {
		if err := writeFull(s.file, record); err != nil {
			return fmt.Errorf("append recovery map journal %s: %w", s.path, err)
		}
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync recovery map journal %s: %w", s.path, err)
	}
	s.pendingBytes = 0
	s.pendingRecords = nil
	s.durableExtents = append([]mapfile.Extent(nil), s.extents...)
	return nil
}

// PendingBytes reports staged journal bytes not yet synced.
