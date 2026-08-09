package mapfile

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const checkpointMagic = "DSCP"

const (
	checkpointFixedPayloadBytes      = uint64(8 + 4)
	defaultMaxCheckpointPayloadBytes = uint64(64 << 20)
	defaultMaxCheckpointExtents      = uint64(1 << 20)
)

// DecodeLimits bounds allocations made while decoding a checkpoint.
type DecodeLimits struct {
	MaxCheckpointPayloadBytes uint64
	MaxCheckpointExtents      uint64
}

// DefaultDecodeLimits are deliberately finite parser limits for v1 maps.
var DefaultDecodeLimits = DecodeLimits{
	MaxCheckpointPayloadBytes: defaultMaxCheckpointPayloadBytes,
	MaxCheckpointExtents:      defaultMaxCheckpointExtents,
}

type Checkpoint struct {
	LastSequence uint64
	Extents      []Extent
}

func MarshalCheckpoint(checkpoint Checkpoint) ([]byte, error) {
	if err := ValidateExtentSet(checkpoint.Extents); err != nil {
		return nil, fmt.Errorf("marshal checkpoint: %w", err)
	}
	if uint64(len(checkpoint.Extents)) > DefaultDecodeLimits.MaxCheckpointExtents {
		return nil, fmt.Errorf("marshal checkpoint: extent count %d exceeds limit %d", len(checkpoint.Extents), DefaultDecodeLimits.MaxCheckpointExtents)
	}

	payloadLength := 8 + 4 + len(checkpoint.Extents)*extentEncodedBytes
	if uint64(payloadLength) > DefaultDecodeLimits.MaxCheckpointPayloadBytes || uint64(payloadLength) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("marshal checkpoint: payload length %d exceeds supported limit", payloadLength)
	}
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
	return UnmarshalCheckpointWithLimits(encoded, DefaultDecodeLimits)
}

// UnmarshalCheckpointWithLimits decodes a checkpoint only after validating its
// declared shape and configured memory limits.
func UnmarshalCheckpointWithLimits(encoded []byte, limits DecodeLimits) (Checkpoint, error) {
	if limits.MaxCheckpointPayloadBytes < checkpointFixedPayloadBytes {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: max payload limit %d is smaller than fixed payload", limits.MaxCheckpointPayloadBytes)
	}
	if len(encoded) < 26 {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: buffer too small")
	}
	if string(encoded[0:4]) != checkpointMagic {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != FormatVersion {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: unsupported version %d", version)
	}
	payloadLength := uint64(binary.LittleEndian.Uint32(encoded[6:10]))
	if payloadLength < checkpointFixedPayloadBytes {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: payload length %d is smaller than fixed payload %d", payloadLength, checkpointFixedPayloadBytes)
	}
	if payloadLength > limits.MaxCheckpointPayloadBytes {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: payload length %d exceeds limit %d", payloadLength, limits.MaxCheckpointPayloadBytes)
	}
	totalLength := uint64(4+2+4+4) + payloadLength
	if uint64(len(encoded)) != totalLength {
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
	count := uint64(binary.LittleEndian.Uint32(encoded[18:22]))
	expectedPayloadLength := checkpointFixedPayloadBytes + count*uint64(extentEncodedBytes)
	if count != 0 && expectedPayloadLength < checkpointFixedPayloadBytes {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: extent count arithmetic overflow")
	}
	if expectedPayloadLength != payloadLength {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: extent count %d requires payload length %d, got %d", count, expectedPayloadLength, payloadLength)
	}
	if count > limits.MaxCheckpointExtents {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: extent count %d exceeds limit %d", count, limits.MaxCheckpointExtents)
	}
	if count > uint64(^uint(0)>>1) {
		return Checkpoint{}, fmt.Errorf("unmarshal checkpoint: extent count %d does not fit implementation", count)
	}
	offset := 22
	checkpoint.Extents = make([]Extent, 0, int(count))
	for i := uint64(0); i < count; i++ {
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
