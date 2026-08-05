package catalog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"time"
)

const journalMagic = "DSCJ"
const maxCatalogPayloadBytes = 1 << 20

var ErrTruncatedCatalogRecord = errors.New("truncated final catalog record")

type EventType uint16

const (
	EventMediaObserved EventType = iota + 1
	EventCaptureStarted
	EventJobLinked
	EventJobStateChanged
	EventFullContentHashAdded
	EventPhysicalLabelSet
	EventPathRelocated
	EventRecordHidden
	EventSnapshotCommitted
)

type CatalogEvent struct {
	Type  EventType
	Entry Entry
}

func (e CatalogEvent) Validate() error {
	if e.Type < EventMediaObserved || e.Type > EventSnapshotCommitted {
		return fmt.Errorf("validate catalog event: unsupported type %d", e.Type)
	}
	if err := e.Entry.Validate(); err != nil {
		return fmt.Errorf("validate catalog event entry: %w", err)
	}
	return nil
}

type JournalRecord struct {
	Type     EventType
	Sequence uint64
	Payload  []byte
}

func (r JournalRecord) Validate() error {
	if r.Sequence == 0 {
		return fmt.Errorf("validate journal record: sequence must be non-zero")
	}
	if len(r.Payload) > maxCatalogPayloadBytes {
		return fmt.Errorf("validate journal record: payload %d exceeds limit %d", len(r.Payload), maxCatalogPayloadBytes)
	}
	if r.Type < EventMediaObserved || r.Type > EventSnapshotCommitted {
		return fmt.Errorf("validate journal record: unsupported type %d", r.Type)
	}
	return nil
}

func MarshalJournalRecord(record JournalRecord) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	encoded := make([]byte, 4+2+2+8+4+len(record.Payload)+4)
	copy(encoded[0:4], []byte(journalMagic))
	binary.LittleEndian.PutUint16(encoded[4:6], catalogFormatVersion)
	binary.LittleEndian.PutUint16(encoded[6:8], uint16(record.Type))
	binary.LittleEndian.PutUint64(encoded[8:16], record.Sequence)
	binary.LittleEndian.PutUint32(encoded[16:20], uint32(len(record.Payload)))
	copy(encoded[20:20+len(record.Payload)], record.Payload)
	crc := crc32.Checksum(encoded[:len(encoded)-4], catalogCRC32CTable)
	binary.LittleEndian.PutUint32(encoded[len(encoded)-4:], crc)
	return encoded, nil
}

func UnmarshalJournalRecord(encoded []byte) (JournalRecord, int, error) {
	if len(encoded) < 24 {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: truncated header")
	}
	if string(encoded[0:4]) != journalMagic {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != catalogFormatVersion {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: unsupported version %d", version)
	}
	payloadLength := int(binary.LittleEndian.Uint32(encoded[16:20]))
	totalLength := 4 + 2 + 2 + 8 + 4 + payloadLength + 4
	if len(encoded) < totalLength {
		return JournalRecord{}, 0, ErrTruncatedCatalogRecord
	}
	if payloadLength > maxCatalogPayloadBytes {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: payload %d exceeds limit %d", payloadLength, maxCatalogPayloadBytes)
	}
	expectedCRC := binary.LittleEndian.Uint32(encoded[totalLength-4 : totalLength])
	actualCRC := crc32.Checksum(encoded[:totalLength-4], catalogCRC32CTable)
	if expectedCRC != actualCRC {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}
	record := JournalRecord{
		Type:     EventType(binary.LittleEndian.Uint16(encoded[6:8])),
		Sequence: binary.LittleEndian.Uint64(encoded[8:16]),
		Payload:  append([]byte(nil), encoded[20:20+payloadLength]...),
	}
	return record, totalLength, record.Validate()
}

func EncodeCatalogEvent(event CatalogEvent, sequence uint64) (JournalRecord, error) {
	if err := event.Validate(); err != nil {
		return JournalRecord{}, err
	}
	payload, err := marshalEntry(event.Entry)
	if err != nil {
		return JournalRecord{}, err
	}
	return JournalRecord{
		Type:     event.Type,
		Sequence: sequence,
		Payload:  payload,
	}, nil
}

func DecodeCatalogEvent(record JournalRecord) (CatalogEvent, error) {
	if err := record.Validate(); err != nil {
		return CatalogEvent{}, err
	}
	entry, err := unmarshalEntry(record.Payload)
	if err != nil {
		return CatalogEvent{}, err
	}
	return CatalogEvent{Type: record.Type, Entry: entry}, nil
}

func AppendJournal(journal []byte, record JournalRecord) ([]byte, error) {
	encoded, err := MarshalJournalRecord(record)
	if err != nil {
		return nil, err
	}
	return append(append([]byte(nil), journal...), encoded...), nil
}

func ReplayJournal(snapshot Snapshot, journal []byte) (Store, error) {
	if err := snapshot.Validate(); err != nil {
		return Store{}, err
	}
	store := Store{
		LastSequence: snapshot.LastSequence,
		Entries:      append([]Entry(nil), snapshot.Entries...),
	}
	offset := 0
	expectedSequence := snapshot.LastSequence + 1
	for offset < len(journal) {
		record, consumed, err := UnmarshalJournalRecord(journal[offset:])
		if err != nil {
			if errors.Is(err, ErrTruncatedCatalogRecord) {
				break
			}
			return Store{}, err
		}
		if record.Sequence < expectedSequence {
			offset += consumed
			continue
		}
		if record.Sequence != expectedSequence {
			return Store{}, fmt.Errorf("replay journal: non-monotonic sequence")
		}
		expectedSequence++
		event, err := DecodeCatalogEvent(record)
		if err != nil {
			return Store{}, err
		}
		store, err = ApplyEvent(store, event, record.Sequence)
		if err != nil {
			return Store{}, err
		}
		offset += consumed
	}
	return store, store.Validate()
}

func ApplyEvent(store Store, event CatalogEvent, sequence uint64) (Store, error) {
	if err := event.Validate(); err != nil {
		return store, err
	}
	next, err := store.UpsertEntry(event.Entry)
	if err != nil {
		return store, err
	}
	next.LastSequence = sequence
	return next, nil
}

func BuildSnapshotRecord(store Store, bounds Bounds) (Snapshot, error) {
	compacted, err := store.Compact(bounds)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		LastSequence: compacted.LastSequence,
		Entries:      append([]Entry(nil), compacted.Entries...),
	}, nil
}

func TruncateJournalAfterSnapshot(journal []byte, snapshot Snapshot) []byte {
	offset := 0
	for offset < len(journal) {
		record, consumed, err := UnmarshalJournalRecord(journal[offset:])
		if err != nil {
			if errors.Is(err, ErrTruncatedCatalogRecord) {
				return nil
			}
			return append([]byte(nil), journal[offset:]...)
		}
		if record.Sequence > snapshot.LastSequence {
			return append([]byte(nil), journal[offset:]...)
		}
		offset += consumed
	}
	return nil
}

func unixNanoUTC(value int64) time.Time {
	return time.Unix(0, value).UTC()
}

func BuildTempSnapshotName(base string) string {
	return base + ".tmp"
}

func ReplaceSnapshotAtomically(current, replacement []byte) []byte {
	return bytes.Clone(replacement)
}
