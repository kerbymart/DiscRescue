package mapfile

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const checkpointMagic = "DSCP"

type Checkpoint struct {
	LastSequence uint64
	Extents      []Extent
}

func MarshalCheckpoint(checkpoint Checkpoint) ([]byte, error) {
	if err := ValidateExtentSet(checkpoint.Extents); err != nil {
		return nil, fmt.Errorf("marshal checkpoint: %w", err)
	}

	payloadLength := 8 + 4 + len(checkpoint.Extents)*extentEncodedBytes
	encoded := make([]byte, 4+2+4+payloadLength+4)
	copy(encoded[0:4], []byte(checkpointMagic))
	binary.LittleEndian.PutUint16(encoded[4:6], FormatVersion)
	binary.LittleEndian.PutUint32(encoded[6:10], uint32(payloadLength))
	binary.LittleEndian.PutUint64(encoded[10:18], checkpoint.LastSequence)
	binary.LittleEndian.PutUint32(encoded[18:22], uint32(len(checkpoint.Extents)))

	offset := 22
	for _, extent := range checkpoint.Extents {
		extentBytes, err := MarshalExtent(extent)
		if err != nil {
			return nil, err
		}
		copy(encoded[offset:offset+extentEncodedBytes], extentBytes)
		offset += extentEncodedBytes
	}

	crc := crc32.Checksum(encoded[:len(encoded)-4], crc32cTable)
	binary.LittleEndian.PutUint32(encoded[len(encoded)-4:], crc)
	return encoded, nil
}

func UnmarshalCheckpoint(encoded []byte) (Checkpoint, error) {
	if len(encoded) < 26 {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: buffer too small")
	}
	if string(encoded[0:4]) != checkpointMagic {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != FormatVersion {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: unsupported version %d", version)
	}
	payloadLength := binary.LittleEndian.Uint32(encoded[6:10])
	totalLength := 4 + 2 + 4 + int(payloadLength) + 4
	if len(encoded) != totalLength {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: length %d does not match expected %d", len(encoded), totalLength)
	}
	expectedCRC := binary.LittleEndian.Uint32(encoded[len(encoded)-4:])
	actualCRC := crc32.Checksum(encoded[:len(encoded)-4], crc32cTable)
	if expectedCRC != actualCRC {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}

	checkpoint := Checkpoint{
		LastSequence: binary.LittleEndian.Uint64(encoded[10:18]),
	}
	count := binary.LittleEndian.Uint32(encoded[18:22])
	offset := 22
	checkpoint.Extents = make([]Extent, 0, count)
	for i := uint32(0); i < count; i++ {
		if offset+extentEncodedBytes > len(encoded)-4 {
			return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: truncated extent %d", i)
		}
		extent, err := UnmarshalExtent(encoded[offset : offset+extentEncodedBytes])
		if err != nil {
			return Checkpoint{}, err
		}
		checkpoint.Extents = append(checkpoint.Extents, extent)
		offset += extentEncodedBytes
	}

	return checkpoint, ValidateExtentSet(checkpoint.Extents)
}
