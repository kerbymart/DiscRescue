package device

import "testing"

func TestValidateFrameRejectsZeroRequestID(t *testing.T) {
	err := ValidateFrame(Frame{RequestID: 0, Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected zero request id to fail validation")
	}
}

func TestValidateFrameRejectsOversizePayload(t *testing.T) {
	err := ValidateFrame(Frame{RequestID: 1, Payload: make([]byte, MaxFramePayloadBytes+1)})
	if err == nil {
		t.Fatal("expected oversize payload to fail validation")
	}
}
