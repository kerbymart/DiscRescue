package device

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const FrameMagic = "DSWP"
const ProtocolVersion uint16 = 1
const MaxFramePayloadBytes = 1 << 20
const frameHeaderBytes = 20
const frameCRCBytes = 4

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

type MessageType uint16

const (
	MessageHello MessageType = iota + 1
	MessageHelloAck
	MessageOpenDevice
	MessageProbeMedia
	MessageReadBlocks
	MessageSetSpeed
	MessageTestReady
	MessageEject
	MessageCloseDevice
	MessageCancelCurrent
	MessageHeartbeat
	MessageResult
	MessageError
	MessageShutdown
)

type Frame struct {
	Type      MessageType
	RequestID uint64
	Payload   []byte
}

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

func MarshalCommandRequest(request CommandRequest) ([]byte, error) {
	commandCode, err := encodeCommandKind(request.Command)
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, 16)
	binary.LittleEndian.PutUint16(encoded[0:2], commandCode)
	binary.LittleEndian.PutUint16(encoded[2:4], 0)
	binary.LittleEndian.PutUint64(encoded[4:12], request.StartLBA)
	binary.LittleEndian.PutUint32(encoded[12:16], request.Sectors)
	return encoded, nil
}

func UnmarshalCommandRequest(payload []byte) (CommandRequest, error) {
	if len(payload) != 16 {
		return CommandRequest{}, fmt.Errorf("unmarshal command request: expected 16 bytes, got %d", len(payload))
	}
	command, err := decodeCommandKind(binary.LittleEndian.Uint16(payload[0:2]))
	if err != nil {
		return CommandRequest{}, err
	}
	return CommandRequest{
		Command:  command,
		StartLBA: binary.LittleEndian.Uint64(payload[4:12]),
		Sectors:  binary.LittleEndian.Uint32(payload[12:16]),
	}, nil
}

// MarshalSetSpeedRequest uses a fixed-width payload so the worker cannot
// receive an unbounded or ambiguous device-control request.
func MarshalSetSpeedRequest(request ReadSpeedRequest) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	encoded := make([]byte, 8)
	if request.Mode == ReadSpeedExplicit {
		encoded[0] = 1
	}
	binary.LittleEndian.PutUint32(encoded[4:8], request.Speed.KilobytesPerSecond)
	return encoded, nil
}

func UnmarshalSetSpeedRequest(payload []byte) (ReadSpeedRequest, error) {
	if len(payload) != 8 {
		return ReadSpeedRequest{}, fmt.Errorf("unmarshal set speed request: expected 8 bytes, got %d", len(payload))
	}
	request := ReadSpeedRequest{Mode: ReadSpeedAuto}
	if payload[0] == 1 {
		request.Mode = ReadSpeedExplicit
	} else if payload[0] != 0 {
		return ReadSpeedRequest{}, fmt.Errorf("unmarshal set speed request: invalid mode %d", payload[0])
	}
	request.Speed.KilobytesPerSecond = binary.LittleEndian.Uint32(payload[4:8])
	if err := request.Validate(); err != nil {
		return ReadSpeedRequest{}, err
	}
	return request, nil
}

func MarshalCommandResult(result CommandResult) ([]byte, error) {
	if len(result.Status) > 0xffff {
		return nil, fmt.Errorf("marshal command result: status length %d exceeds uint16", len(result.Status))
	}
	if len(result.Data) > MaxFramePayloadBytes {
		return nil, fmt.Errorf("marshal command result: data length %d exceeds payload limit %d", len(result.Data), MaxFramePayloadBytes)
	}

	encoded := make([]byte, 2+len(result.Status)+4+len(result.Data))
	binary.LittleEndian.PutUint16(encoded[0:2], uint16(len(result.Status)))
	copy(encoded[2:2+len(result.Status)], []byte(result.Status))
	dataOffset := 2 + len(result.Status)
	binary.LittleEndian.PutUint32(encoded[dataOffset:dataOffset+4], uint32(len(result.Data)))
	copy(encoded[dataOffset+4:], result.Data)
	return encoded, nil
}

func UnmarshalCommandResult(payload []byte) (CommandResult, error) {
	if len(payload) < 6 {
		return CommandResult{}, fmt.Errorf("unmarshal command result: payload too short")
	}
	statusLength := int(binary.LittleEndian.Uint16(payload[0:2]))
	if len(payload) < 2+statusLength+4 {
		return CommandResult{}, fmt.Errorf("unmarshal command result: truncated status or data length")
	}
	status := string(payload[2 : 2+statusLength])
	dataOffset := 2 + statusLength
	dataLength := int(binary.LittleEndian.Uint32(payload[dataOffset : dataOffset+4]))
	if len(payload) != dataOffset+4+dataLength {
		return CommandResult{}, fmt.Errorf("unmarshal command result: payload length %d does not match expected length %d", len(payload), dataOffset+4+dataLength)
	}
	return CommandResult{
		Status: status,
		Data:   append([]byte(nil), payload[dataOffset+4:]...),
	}, nil
}

func encodeCommandKind(kind CommandKind) (uint16, error) {
	switch kind {
	case CommandInquiry:
		return 1, nil
	case CommandReadBlocks:
		return 2, nil
	default:
		return 0, fmt.Errorf("encode command kind: unsupported command %q", kind)
	}
}

func decodeCommandKind(code uint16) (CommandKind, error) {
	switch code {
	case 1:
		return CommandInquiry, nil
	case 2:
		return CommandReadBlocks, nil
	default:
		return "", fmt.Errorf("decode command kind: unsupported code %d", code)
	}
}
