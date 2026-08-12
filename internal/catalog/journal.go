package catalog

import (
	"errors"
	"fmt"
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
