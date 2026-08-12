package device

import (
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
