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
	path           string
	file           *os.File
	header         mapfile.Header
	extents        []mapfile.Extent
	durableExtents []mapfile.Extent
	nextSequence   uint64
	pendingBytes   uint64
	pendingRecords [][]byte
	closed         bool
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

// DurableExtents returns the last extent snapshot confirmed by a journal
// sync. It may lag Extents while a bounded durability batch is pending.
func (s *Store) DurableExtents() []mapfile.Extent {
	if s == nil {
		return nil
	}
	return append([]mapfile.Extent(nil), s.durableExtents...)
}

// ApplyExtent durably appends one extent transition using explicit EOF
// positioning. The map is never opened with O_APPEND so header WriteAt remains
// valid during finalization.
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
func (s *Store) PendingBytes() uint64 {
	if s == nil {
		return 0
	}
	return s.pendingBytes
}

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

func writeFull(file *os.File, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

// Close finalizes the session header and closes the map. A clean state is
// recorded only after the header write and map sync both succeed.
func (s *Store) Close(clean bool) error {
	if s == nil || s.file == nil || s.closed {
		return nil
	}
	if err := s.Flush(); err != nil {
		closeErr := s.file.Close()
		s.closed = true
		return errors.Join(err, closeErr)
	}
	s.header.CleanShutdown = clean
	headerBytes, err := mapfile.MarshalHeader(s.header)
	if err != nil {
		return s.failClose(fmt.Errorf("finalize recovery map %s: %w", s.path, err))
	}
	if err := s.writeAt(0, headerBytes); err != nil {
		return s.failClose(fmt.Errorf("finalize recovery map %s: %w", s.path, err))
	}
	if err := s.file.Sync(); err != nil {
		closeErr := s.file.Close()
		s.closed = true
		return errors.Join(fmt.Errorf("sync finalized recovery map %s: %w", s.path, err), closeErr)
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

func (s *Store) abortCreate(cause error) error {
	closeErr := s.file.Close()
	removeErr := os.Remove(s.path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(cause, closeErr, removeErr)
}

func (s *Store) failClose(cause error) error {
	closeErr := s.file.Close()
	s.closed = true
	return errors.Join(cause, closeErr)
}

func readState(path string, geometry Geometry) (mapfile.Header, mapfile.Checkpoint, error) {
	if geometry.LogicalSectorSize == 0 || geometry.ExpectedSectorCount == 0 {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("media geometry is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("open: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("stat: %w", err)
	}
	if info.Size() < 8 {
		return mapfile.Header{}, mapfile.Checkpoint{}, errors.New("recovery map is too short")
	}
	prefix := make([]byte, 8)
	if err := readAtExact(file, 0, prefix); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read prefix: %w", err)
	}
	headerLength := int(binary.LittleEndian.Uint16(prefix[6:8]))
	if headerLength <= 0 {
		return mapfile.Header{}, mapfile.Checkpoint{}, errors.New("recovery map header length is invalid")
	}
	headerBytes := make([]byte, headerLength)
	if err := readAtExact(file, 0, headerBytes); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read header: %w", err)
	}
	header, err := mapfile.UnmarshalHeader(headerBytes)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	checkpointPrefix := make([]byte, 10)
	if err := readAtExact(file, int64(headerLength), checkpointPrefix); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read checkpoint header: %w", err)
	}
	payloadLength := uint64(binary.LittleEndian.Uint32(checkpointPrefix[6:10]))
	checkpointLength := uint64(14) + payloadLength
	if checkpointLength > uint64(info.Size())-uint64(headerLength) {
		return mapfile.Header{}, mapfile.Checkpoint{}, errors.New("recovery map checkpoint is truncated")
	}
	if payloadLength > mapfile.DefaultDecodeLimits.MaxCheckpointPayloadBytes {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("recovery map checkpoint payload %d exceeds limit", payloadLength)
	}
	checkpointBytes := make([]byte, int(checkpointLength))
	if err := readAtExact(file, int64(headerLength), checkpointBytes); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read checkpoint: %w", err)
	}
	checkpoint, err := mapfile.UnmarshalCheckpoint(checkpointBytes)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	journalOffset := uint64(headerLength) + checkpointLength
	if header.LogicalSectorSize != geometry.LogicalSectorSize || header.ExpectedSectorCount != geometry.ExpectedSectorCount {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("map geometry does not match media")
	}
	journalReader := io.NewSectionReader(file, int64(journalOffset), info.Size()-int64(journalOffset))
	replayed, err := mapfile.ReplayJournalReaderWithinCapacity(checkpoint, journalReader, geometry.ExpectedSectorCount)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	return header, replayed, nil
}

func readAtExact(file *os.File, offset int64, data []byte) error {
	read, err := file.ReadAt(data, offset)
	if err != nil {
		return err
	}
	if read != len(data) {
		return io.ErrUnexpectedEOF
	}
	return nil
}
