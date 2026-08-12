package mapfile

import "fmt"

const maxJournalPayloadBytes = 1 << 20

// MaxJournalPayloadBytes is the maximum payload a journal record may carry.
// Readers use it to bound record buffers before reading untrusted lengths.
const MaxJournalPayloadBytes = maxJournalPayloadBytes

type JournalRecord struct {
	Type     RecordType
	Sequence uint64
	Extent   *Extent
	Payload  []byte
}

func (r JournalRecord) Validate() error {
	if r.Sequence == 0 {
		return fmt.Errorf("validate journal record: sequence must be non-zero")
	}
	payload, err := r.payloadBytes()
	if err != nil {
		return err
	}
	if len(payload) > maxJournalPayloadBytes {
		return fmt.Errorf("validate journal record: payload %d exceeds limit %d", len(payload), maxJournalPayloadBytes)
	}
	return nil
}

func (r JournalRecord) payloadBytes() ([]byte, error) {
	if r.Extent != nil {
		return MarshalExtent(*r.Extent)
	}
	return append([]byte(nil), r.Payload...), nil
}
