package device

import "fmt"

const MaxFramePayloadBytes = 1 << 20

type Frame struct {
	RequestID uint64
	Payload   []byte
}

func ValidateFrame(frame Frame) error {
	if len(frame.Payload) > MaxFramePayloadBytes {
		return fmt.Errorf("validate frame: payload %d exceeds limit %d", len(frame.Payload), MaxFramePayloadBytes)
	}
	return nil
}
