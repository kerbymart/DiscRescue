// Package recoverymap owns the platform-neutral durable lifecycle of a
// DiscRescue recovery map.
package recoverymap

import (
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

func (s *Store) PendingBytes() uint64 {
	if s == nil {
		return 0
	}
	return s.pendingBytes
}
