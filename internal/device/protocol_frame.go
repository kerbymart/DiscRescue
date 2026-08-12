package device

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

func ValidateFrame(frame Frame) error {
	if frame.Type == 0 {
		return fmt.Errorf("validate frame: type must be non-zero")
	}
	if frame.RequestID == 0 {
		return fmt.Errorf("validate frame: request id must be non-zero")
	}
	if len(frame.Payload) > MaxFramePayloadBytes {
		return fmt.Errorf("validate frame: payload %d exceeds limit %d", len(frame.Payload), MaxFramePayloadBytes)
	}
	return nil
}
func MarshalFrame(frame Frame) ([]byte, error) {
	if err := ValidateFrame(frame); err != nil {
		return nil, err
	}

	encoded := make([]byte, frameHeaderBytes+len(frame.Payload)+frameCRCBytes)
	copy(encoded[0:4], []byte(FrameMagic))
	binary.LittleEndian.PutUint16(encoded[4:6], ProtocolVersion)
	binary.LittleEndian.PutUint16(encoded[6:8], uint16(frame.Type))
	binary.LittleEndian.PutUint64(encoded[8:16], frame.RequestID)
	binary.LittleEndian.PutUint32(encoded[16:20], uint32(len(frame.Payload)))
	copy(encoded[20:20+len(frame.Payload)], frame.Payload)

	crc := crc32.Checksum(encoded[:len(encoded)-frameCRCBytes], crc32cTable)
	binary.LittleEndian.PutUint32(encoded[len(encoded)-frameCRCBytes:], crc)
	return encoded, nil
}
func UnmarshalFrame(encoded []byte) (Frame, error) {
	if len(encoded) < frameHeaderBytes+frameCRCBytes {
		return Frame{}, fmt.Errorf("unmarshal frame: buffer length %d is too small", len(encoded))
	}
	if string(encoded[0:4]) != FrameMagic {
		return Frame{}, fmt.Errorf("unmarshal frame: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != ProtocolVersion {
		return Frame{}, fmt.Errorf("unmarshal frame: unsupported version %d", version)
	}

	payloadLength := binary.LittleEndian.Uint32(encoded[16:20])
	if payloadLength > MaxFramePayloadBytes {
		return Frame{}, fmt.Errorf("unmarshal frame: payload %d exceeds limit %d", payloadLength, MaxFramePayloadBytes)
	}

	expectedLength := frameHeaderBytes + int(payloadLength) + frameCRCBytes
	if len(encoded) != expectedLength {
		return Frame{}, fmt.Errorf("unmarshal frame: buffer length %d does not match expected length %d", len(encoded), expectedLength)
	}

	expectedCRC := binary.LittleEndian.Uint32(encoded[len(encoded)-frameCRCBytes:])
	actualCRC := crc32.Checksum(encoded[:len(encoded)-frameCRCBytes], crc32cTable)
	if expectedCRC != actualCRC {
		return Frame{}, fmt.Errorf("unmarshal frame: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}

	frame := Frame{
		Type:      MessageType(binary.LittleEndian.Uint16(encoded[6:8])),
		RequestID: binary.LittleEndian.Uint64(encoded[8:16]),
		Payload:   append([]byte(nil), encoded[20:20+payloadLength]...),
	}
	if err := ValidateFrame(frame); err != nil {
		return Frame{}, err
	}
	return frame, nil
}
