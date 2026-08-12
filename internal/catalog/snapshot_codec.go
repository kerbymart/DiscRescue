package catalog

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

func MarshalSnapshot(snapshot Snapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	payload, err := marshalEntries(snapshot.Entries)
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, 4+2+4+8+4+len(payload)+4)
	copy(encoded[0:4], []byte(snapshotMagic))
	binary.LittleEndian.PutUint16(encoded[4:6], catalogFormatVersion)
	binary.LittleEndian.PutUint32(encoded[6:10], uint32(len(payload)))
	binary.LittleEndian.PutUint64(encoded[10:18], snapshot.LastSequence)
	binary.LittleEndian.PutUint32(encoded[18:22], uint32(len(snapshot.Entries)))
	copy(encoded[22:22+len(payload)], payload)
	crc := crc32.Checksum(encoded[:len(encoded)-4], catalogCRC32CTable)
	binary.LittleEndian.PutUint32(encoded[len(encoded)-4:], crc)
	return encoded, nil
}
func UnmarshalSnapshot(encoded []byte) (Snapshot, error) {
	if len(encoded) < 26 {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: buffer too small")
	}
	if string(encoded[0:4]) != snapshotMagic {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != catalogFormatVersion {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: unsupported version %d", version)
	}
	payloadLength := int(binary.LittleEndian.Uint32(encoded[6:10]))
	expectedLength := 4 + 2 + 4 + 8 + 4 + payloadLength + 4
	if len(encoded) != expectedLength {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: length %d does not match expected %d", len(encoded), expectedLength)
	}
	expectedCRC := binary.LittleEndian.Uint32(encoded[len(encoded)-4:])
	actualCRC := crc32.Checksum(encoded[:len(encoded)-4], catalogCRC32CTable)
	if expectedCRC != actualCRC {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}

	entryCount := int(binary.LittleEndian.Uint32(encoded[18:22]))
	if entryCount > maxCatalogEntries {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: entry count %d exceeds limit %d", entryCount, maxCatalogEntries)
	}
	if uint64(entryCount)*4 > uint64(payloadLength) {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: entry count %d cannot fit payload", entryCount)
	}
	entries, err := unmarshalEntries(encoded[22:22+payloadLength], entryCount)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		LastSequence: binary.LittleEndian.Uint64(encoded[10:18]),
		Entries:      entries,
	}
	return snapshot, snapshot.Validate()
}
