// Package recoverymap owns the platform-neutral durable lifecycle of a
// DiscRescue recovery map.
package recoverymap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"discrescue/internal/mapfile"
)

// Geometry identifies the media a map is allowed to describe.
type Geometry struct {
	LogicalSectorSize   uint32
	ExpectedSectorCount uint64
}

// Store owns one recovery map file and its in-memory working extent state.
// Store is intentionally single-owner; callers must serialize its methods.
type Store struct {
	path         string
	file         *os.File
	header       mapfile.Header
	extents      []mapfile.Extent
	nextSequence uint64
	closed       bool
}

// Create creates and durably initializes a new recovery map.
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
		return nil, store.abort(err)
	}
	if err := store.writeAt(int64(len(headerBytes)), checkpointBytes); err != nil {
		return nil, store.abort(err)
	}
	if err := file.Sync(); err != nil {
		return nil, store.abort(fmt.Errorf("sync recovery map %s: %w", path, err))
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return nil, store.abort(fmt.Errorf("seek recovery map journal %s: %w", path, err))
	}
	return store, nil
}

// Open validates and opens an existing recovery map for a writable session.
// Existing files are never removed or truncated by Open.
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
		path:         path,
		file:         file,
		header:       header,
		extents:      append([]mapfile.Extent(nil), replayed.Extents...),
		nextSequence: replayed.LastSequence + 1,
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
func Inspect(path string, geometry Geometry) (mapfile.Header, []mapfile.Extent, error) {
	header, replayed, err := readState(path, geometry)
	if err != nil {
		return mapfile.Header{}, nil, fmt.Errorf("inspect recovery map %s: %w", path, err)
	}
	return header, append([]mapfile.Extent(nil), replayed.Extents...), nil
}

// Path returns the map's filesystem path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Header returns the current map header.
func (s *Store) Header() mapfile.Header {
	if s == nil {
		return mapfile.Header{}
	}
	return s.header
}

// Extents returns an immutable snapshot of the working extent state.
func (s *Store) Extents() []mapfile.Extent {
	if s == nil {
		return nil
	}
	return append([]mapfile.Extent(nil), s.extents...)
}

// ApplyExtent durably appends one extent transition using explicit EOF
// positioning. The map is never opened with O_APPEND so header WriteAt remains
// valid during finalization.
func (s *Store) ApplyExtent(extent mapfile.Extent) error {
	if s == nil || s.file == nil || s.closed {
		return errors.New("apply recovery map extent: store is closed")
	}
	nextExtents, err := mapfile.ApplyExtent(s.extents, extent)
	if err != nil {
		return fmt.Errorf("apply recovery map extent: %w", err)
	}
	if s.nextSequence == 0 {
		return errors.New("apply recovery map extent: sequence exhausted")
	}
	record, err := mapfile.MarshalJournalRecord(mapfile.JournalRecord{
		Type:     mapfile.RecordExtentStateChanged,
		Sequence: s.nextSequence,
		Extent:   &extent,
	})
	if err != nil {
		return fmt.Errorf("marshal recovery map extent: %w", err)
	}
	if _, err := s.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek recovery map journal %s: %w", s.path, err)
	}
	if _, err := s.file.Write(record); err != nil {
		return fmt.Errorf("append recovery map journal %s: %w", s.path, err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync recovery map journal %s: %w", s.path, err)
	}
	s.extents = nextExtents
	s.nextSequence++
	return nil
}

// Close finalizes the session header and closes the map. A clean state is
// recorded only after the header write and map sync both succeed.
func (s *Store) Close(clean bool) error {
	if s == nil || s.file == nil || s.closed {
		return nil
	}
	s.header.CleanShutdown = clean
	headerBytes, err := mapfile.MarshalHeader(s.header)
	if err != nil {
		return fmt.Errorf("finalize recovery map %s: %w", s.path, err)
	}
	if err := s.writeAt(0, headerBytes); err != nil {
		return fmt.Errorf("finalize recovery map %s: %w", s.path, err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync finalized recovery map %s: %w", s.path, err)
	}
	s.closed = true
	return s.file.Close()
}

func (s *Store) writeAt(offset int64, data []byte) error {
	written, err := s.file.WriteAt(data, offset)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func (s *Store) abort(cause error) error {
	closeErr := s.file.Close()
	return errors.Join(cause, closeErr)
}

func decode(data []byte) (mapfile.Header, mapfile.Checkpoint, int, error) {
	if len(data) < 8 {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, errors.New("recovery map is too short")
	}
	headerLength := int(binary.LittleEndian.Uint16(data[6:8]))
	if headerLength <= 0 || len(data) < headerLength+10 {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, errors.New("recovery map is missing checkpoint data")
	}
	header, err := mapfile.UnmarshalHeader(data[:headerLength])
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, err
	}
	payloadLength := uint64(binary.LittleEndian.Uint32(data[headerLength+6 : headerLength+10]))
	checkpointLength := uint64(14) + payloadLength
	if checkpointLength > uint64(len(data)-headerLength) {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, errors.New("recovery map checkpoint is truncated")
	}
	checkpointEnd := uint64(headerLength) + checkpointLength
	checkpoint, err := mapfile.UnmarshalCheckpoint(data[headerLength:int(checkpointEnd)])
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, 0, err
	}
	return header, checkpoint, int(checkpointEnd), nil
}

func readState(path string, geometry Geometry) (mapfile.Header, mapfile.Checkpoint, error) {
	if geometry.LogicalSectorSize == 0 || geometry.ExpectedSectorCount == 0 {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("media geometry is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read: %w", err)
	}
	header, checkpoint, journalOffset, err := decode(data)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	if header.LogicalSectorSize != geometry.LogicalSectorSize || header.ExpectedSectorCount != geometry.ExpectedSectorCount {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("map geometry does not match media")
	}
	replayed, err := mapfile.ReplayJournalWithinCapacity(checkpoint, data[journalOffset:], geometry.ExpectedSectorCount)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	return header, replayed, nil
}
