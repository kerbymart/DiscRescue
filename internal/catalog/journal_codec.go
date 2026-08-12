package catalog

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

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
	return JournalRecord{Type: event.Type, Sequence: sequence, Payload: payload}, nil
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
