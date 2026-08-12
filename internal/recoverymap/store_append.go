package recoverymap

import (
	"errors"
	"fmt"
	"io"

	"discrescue/internal/mapfile"
)

func (s *Store) ApplyExtent(extent mapfile.Extent) error {
	if s == nil || s.file == nil || s.closed {
		return errors.New("apply recovery map extent: store is closed")
	}
	if err := s.Flush(); err != nil {
		return fmt.Errorf("flush pending recovery map journal: %w", err)
	}
	return s.appendExtent(extent, true)
}

// StageExtent appends an extent transition without forcing a sync. The caller
// must call Flush before relying on the transition after a crash.
func (s *Store) StageExtent(extent mapfile.Extent) error {
	if s == nil || s.file == nil || s.closed {
		return errors.New("stage recovery map extent: store is closed")
	}
	nextExtents, record, err := s.prepareExtent(extent)
	if err != nil {
		return err
	}
	s.extents = nextExtents
	s.nextSequence++
	s.pendingRecords = append(s.pendingRecords, record)
	s.pendingBytes += uint64(len(record))
	return nil
}

// Flush durably syncs staged journal transitions.
func (s *Store) appendExtent(extent mapfile.Extent, sync bool) error {
	nextExtents, record, err := s.prepareExtent(extent)
	if err != nil {
		return err
	}
	if _, err := s.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek recovery map journal %s: %w", s.path, err)
	}
	if err := writeFull(s.file, record); err != nil {
		return fmt.Errorf("append recovery map journal %s: %w", s.path, err)
	}
	if sync {
		if err := s.file.Sync(); err != nil {
			return fmt.Errorf("sync recovery map journal %s: %w", s.path, err)
		}
	}
	s.extents = nextExtents
	s.nextSequence++
	if sync {
		s.durableExtents = append([]mapfile.Extent(nil), s.extents...)
	}
	return nil
}
func (s *Store) prepareExtent(extent mapfile.Extent) ([]mapfile.Extent, []byte, error) {
	nextExtents, err := mapfile.ApplyExtent(s.extents, extent)
	if err != nil {
		return nil, nil, fmt.Errorf("validate recovery map extent: %w", err)
	}
	if s.nextSequence == 0 {
		return nil, nil, errors.New("apply recovery map extent: sequence exhausted")
	}
	record, err := mapfile.MarshalJournalRecord(mapfile.JournalRecord{
		Type:     mapfile.RecordExtentStateChanged,
		Sequence: s.nextSequence,
		Extent:   &extent,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal recovery map extent: %w", err)
	}
	return nextExtents, record, nil
}
