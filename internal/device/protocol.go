package device

import "fmt"

const FrameMagic = "DSWP"
const ProtocolVersion uint16 = 1
const MaxFramePayloadBytes = 1 << 20

type Frame struct {
	RequestID uint64
	Payload   []byte
}

func ValidateFrame(frame Frame) error {
	if frame.RequestID == 0 {
		return fmt.Errorf("validate frame: request id must be non-zero")
	}
	if len(frame.Payload) > MaxFramePayloadBytes {
		return fmt.Errorf("validate frame: payload %d exceeds limit %d", len(frame.Payload), MaxFramePayloadBytes)
	}
	return nil
}
