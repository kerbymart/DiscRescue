package mapfile

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

func MarshalJournalRecord(record JournalRecord) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	payload, err := record.payloadBytes()
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, 2+8+4+len(payload)+4)
	binary.LittleEndian.PutUint16(encoded[0:2], uint16(recordTypeCode(record.Type)))
	binary.LittleEndian.PutUint64(encoded[2:10], record.Sequence)
	binary.LittleEndian.PutUint32(encoded[10:14], uint32(len(payload)))
	copy(encoded[14:14+len(payload)], payload)
	crc := crc32.Checksum(encoded[:len(encoded)-4], crc32cTable)
	binary.LittleEndian.PutUint32(encoded[len(encoded)-4:], crc)
	return encoded, nil
}

func UnmarshalJournalRecord(encoded []byte) (JournalRecord, int, error) {
	if len(encoded) < 18 {
		return JournalRecord{}, 0, ErrTruncatedRecord
	}

	payloadLength := binary.LittleEndian.Uint32(encoded[10:14])
	totalLength := 2 + 8 + 4 + int(payloadLength) + 4
	if len(encoded) < totalLength {
		return JournalRecord{}, 0, ErrTruncatedRecord
	}
	if payloadLength > maxJournalPayloadBytes {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: payload %d exceeds limit %d", payloadLength, maxJournalPayloadBytes)
	}

	expectedCRC := binary.LittleEndian.Uint32(encoded[totalLength-4 : totalLength])
	actualCRC := crc32.Checksum(encoded[:totalLength-4], crc32cTable)
	if expectedCRC != actualCRC {
		return JournalRecord{}, 0, fmt.Errorf("unmarshal journal record: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}

	recordType, err := decodeRecordType(binary.LittleEndian.Uint16(encoded[0:2]))
	if err != nil {
		return JournalRecord{}, 0, err
	}

	record := JournalRecord{Type: recordType, Sequence: binary.LittleEndian.Uint64(encoded[2:10]), Payload: append([]byte(nil), encoded[14:totalLength-4]...)}
	if record.Type == RecordExtentStateChanged {
		extent, err := UnmarshalExtent(record.Payload)
		if err != nil {
			return JournalRecord{}, 0, err
		}
		record.Extent = &extent
		record.Payload = nil
	}

	return record, totalLength, record.Validate()
}

func AppendJournalRecord(journal []byte, record JournalRecord) ([]byte, error) {
	encoded, err := MarshalJournalRecord(record)
	if err != nil {
		return nil, err
	}
	return append(append([]byte(nil), journal...), encoded...), nil
}

func recordTypeCode(recordType RecordType) uint16 {
	switch recordType {
	case RecordJobCreated:
		return 1
	case RecordCaptureOpened:
		return 2
	case RecordPassStarted:
		return 3
	case RecordDataWritten:
		return 4
	case RecordExtentStateChanged:
		return 5
	case RecordErrorRecorded:
		return 6
	case RecordMediaReidentified:
		return 7
	case RecordCheckpointCommitted:
		return 8
	case RecordPassFinished:
		return 9
	case RecordJobStopped:
		return 10
	case RecordJobCompleted:
		return 11
	default:
		return 0
	}
}

func decodeRecordType(code uint16) (RecordType, error) {
	switch code {
	case 1:
		return RecordJobCreated, nil
	case 2:
		return RecordCaptureOpened, nil
	case 3:
		return RecordPassStarted, nil
	case 4:
		return RecordDataWritten, nil
	case 5:
		return RecordExtentStateChanged, nil
	case 6:
		return RecordErrorRecorded, nil
	case 7:
		return RecordMediaReidentified, nil
	case 8:
		return RecordCheckpointCommitted, nil
	case 9:
		return RecordPassFinished, nil
	case 10:
		return RecordJobStopped, nil
	case 11:
		return RecordJobCompleted, nil
	default:
		return "", fmt.Errorf("decode record type: unsupported code %d", code)
	}
}
