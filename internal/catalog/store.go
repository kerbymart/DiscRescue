package catalog

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
)

type RecordID [16]byte

type JobReference struct {
	JobID        RecordID
	Path         string
	FilesPresent bool
}

func (r JobReference) Validate() error {
	if r.JobID == (RecordID{}) {
		return fmt.Errorf("validate job reference: job id is required")
	}
	if r.Path == "" {
		return fmt.Errorf("validate job reference: path is required")
	}
	return nil
}

type Entry struct {
	RecordID              RecordID
	Identity              ContentIdentity
	State                 ProcessingState
	Status                string
	FirstSeenUnixNano     int64
	LastSeenUnixNano      int64
	LastProcessedPresent  bool
	LastProcessedUnixNano int64
	Captures              []CaptureIdentity
	JobReferences         []JobReference
	PreferredJobID        RecordID
	Hidden                bool
}

func (e Entry) Validate() error {
	if e.RecordID == (RecordID{}) {
		return fmt.Errorf("validate entry: record id is required")
	}
	if err := e.Identity.Validate(); err != nil {
		return fmt.Errorf("validate entry identity: %w", err)
	}
	if !validProcessingState(e.State) {
		return fmt.Errorf("validate entry: unsupported processing state %q", e.State)
	}
	if e.LastSeenUnixNano < e.FirstSeenUnixNano {
		return fmt.Errorf("validate entry: last seen time %d is earlier than first seen time %d", e.LastSeenUnixNano, e.FirstSeenUnixNano)
	}
	if !e.LastProcessedPresent && e.LastProcessedUnixNano != 0 {
		return fmt.Errorf("validate entry: last processed timestamp must be zero when absent")
	}
	for i, capture := range e.Captures {
		if err := capture.Validate(); err != nil {
			return fmt.Errorf("validate entry capture[%d]: %w", i, err)
		}
	}
	for i, reference := range e.JobReferences {
		if err := reference.Validate(); err != nil {
			return fmt.Errorf("validate entry job reference[%d]: %w", i, err)
		}
	}
	return nil
}

type Bounds struct {
	MaxEntries          int
	MaxCapturesPerEntry int
	MaxJobRefsPerEntry  int
}

func (b Bounds) Validate() error {
	if b.MaxEntries < 0 || b.MaxCapturesPerEntry < 0 || b.MaxJobRefsPerEntry < 0 {
		return fmt.Errorf("validate bounds: limits must be non-negative")
	}
	return nil
}

type OpenMode string

const (
	OpenReadOnly OpenMode = "read_only"
	OpenMutable  OpenMode = "mutable"
)

type OpenResult struct {
	Mode                    OpenMode
	HistoryUpdatesAvailable bool
}

type Store struct {
	LastSequence     uint64
	Entries          []Entry
	MutationLockHeld bool
}

func (s Store) Validate() error {
	for i, entry := range s.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("validate store entry[%d]: %w", i, err)
		}
	}
	return nil
}

func (s Store) AcquireMutationLock() (Store, error) {
	if s.MutationLockHeld {
		return s, fmt.Errorf("acquire mutation lock: lock is already held")
	}
	next := s
	next.MutationLockHeld = true
	return next, nil
}

func (s Store) ReleaseMutationLock() Store {
	next := s
	next.MutationLockHeld = false
	return next
}

func (s Store) Open(preferMutable bool) (OpenResult, error) {
	if preferMutable {
		if s.MutationLockHeld {
			return OpenResult{}, fmt.Errorf("open store: mutation lock is already held")
		}
		return OpenResult{Mode: OpenMutable, HistoryUpdatesAvailable: true}, nil
	}
	return OpenResult{
		Mode:                    OpenReadOnly,
		HistoryUpdatesAvailable: !s.MutationLockHeld,
	}, nil
}

func (s Store) UpsertEntry(entry Entry) (Store, error) {
	if err := entry.Validate(); err != nil {
		return s, err
	}
	next := s
	replaced := false
	for i := range next.Entries {
		if next.Entries[i].RecordID == entry.RecordID {
			next.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		next.Entries = append(next.Entries, entry)
	}
	return next, nil
}

func (s Store) Compact(bounds Bounds) (Store, error) {
	if err := s.Validate(); err != nil {
		return Store{}, err
	}
	if err := bounds.Validate(); err != nil {
		return Store{}, err
	}

	next := s
	next.Entries = append([]Entry(nil), s.Entries...)
	sort.Slice(next.Entries, func(i, j int) bool {
		if next.Entries[i].LastSeenUnixNano != next.Entries[j].LastSeenUnixNano {
			return next.Entries[i].LastSeenUnixNano > next.Entries[j].LastSeenUnixNano
		}
		return bytes.Compare(next.Entries[i].RecordID[:], next.Entries[j].RecordID[:]) < 0
	})
	if bounds.MaxEntries > 0 && len(next.Entries) > bounds.MaxEntries {
		next.Entries = next.Entries[:bounds.MaxEntries]
	}
	for i := range next.Entries {
		if bounds.MaxCapturesPerEntry > 0 && len(next.Entries[i].Captures) > bounds.MaxCapturesPerEntry {
			next.Entries[i].Captures = append([]CaptureIdentity(nil), next.Entries[i].Captures[len(next.Entries[i].Captures)-bounds.MaxCapturesPerEntry:]...)
		}
		if bounds.MaxJobRefsPerEntry > 0 && len(next.Entries[i].JobReferences) > bounds.MaxJobRefsPerEntry {
			next.Entries[i].JobReferences = append([]JobReference(nil), next.Entries[i].JobReferences[len(next.Entries[i].JobReferences)-bounds.MaxJobRefsPerEntry:]...)
		}
	}
	return next, nil
}

func validProcessingState(state ProcessingState) bool {
	switch state {
	case ProcessingObserved,
		ProcessingInProgress,
		ProcessingStoppedResumable,
		ProcessingCompletedVerified,
		ProcessingCompletedWithGaps,
		ProcessingFailed,
		ProcessingMerged:
		return true
	default:
		return false
	}
}

func encodeString(buf *bytes.Buffer, value string) error {
	if len(value) > 0xffff {
		return fmt.Errorf("encode string: length %d exceeds uint16", len(value))
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(value))); err != nil {
		return err
	}
	_, err := buf.WriteString(value)
	return err
}

func decodeString(data []byte, offset *int) (string, error) {
	if *offset+2 > len(data) {
		return "", fmt.Errorf("decode string: truncated length")
	}
	length := int(binary.LittleEndian.Uint16(data[*offset : *offset+2]))
	*offset += 2
	if *offset+length > len(data) {
		return "", fmt.Errorf("decode string: truncated value")
	}
	value := string(data[*offset : *offset+length])
	*offset += length
	return value, nil
}

func encodeBool(buf *bytes.Buffer, value bool) error {
	var encoded byte
	if value {
		encoded = 1
	}
	return buf.WriteByte(encoded)
}

func decodeBool(data []byte, offset *int) (bool, error) {
	if *offset+1 > len(data) {
		return false, fmt.Errorf("decode bool: truncated value")
	}
	value := data[*offset]
	*offset++
	if value > 1 {
		return false, fmt.Errorf("decode bool: invalid value %d", value)
	}
	return value == 1, nil
}
