package device

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestValidateFrameRejectsZeroRequestID(t *testing.T) {
	err := ValidateFrame(Frame{Type: MessageHello, RequestID: 0, Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected zero request id to fail validation")
	}
}

func TestValidateFrameRejectsOversizePayload(t *testing.T) {
	err := ValidateFrame(Frame{Type: MessageHello, RequestID: 1, Payload: make([]byte, MaxFramePayloadBytes+1)})
	if err == nil {
		t.Fatal("expected oversize payload to fail validation")
	}
}

func TestMarshalFrameMatchesFixedVector(t *testing.T) {
	frame := Frame{
		Type:      MessageHelloAck,
		RequestID: 7,
		Payload:   []byte{0x10, 0x20},
	}

	encoded, err := MarshalFrame(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}

	expectedHex := "4453575001000200070000000000000002000000102046bbc66c"
	if hex.EncodeToString(encoded) != expectedHex {
		t.Fatalf("unexpected frame bytes: got %s want %s", hex.EncodeToString(encoded), expectedHex)
	}
}

func TestUnmarshalFrameRejectsCorruptCRC(t *testing.T) {
	encoded, err := MarshalFrame(Frame{
		Type:      MessageHeartbeat,
		RequestID: 9,
		Payload:   []byte("ok"),
	})
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	encoded[len(encoded)-1] ^= 0xff

	if _, err := UnmarshalFrame(encoded); err == nil {
		t.Fatal("expected corrupt crc to fail")
	}
}

func TestMarshalAndUnmarshalCommandRequest(t *testing.T) {
	request := CommandRequest{
		Command:  CommandReadBlocks,
		StartLBA: 1234,
		Sectors:  16,
	}

	encoded, err := MarshalCommandRequest(request)
	if err != nil {
		t.Fatalf("marshal command request: %v", err)
	}

	expected := []byte{
		0x02, 0x00, 0x00, 0x00,
		0xd2, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x10, 0x00, 0x00, 0x00,
	}
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("unexpected command request bytes: got %x want %x", encoded, expected)
	}

	decoded, err := UnmarshalCommandRequest(expected)
	if err != nil {
		t.Fatalf("unmarshal command request: %v", err)
	}
	if decoded.Command != request.Command || decoded.StartLBA != request.StartLBA || decoded.Sectors != request.Sectors {
		t.Fatalf("unexpected decoded request: got %+v want %+v", decoded, request)
	}
}

func TestMarshalAndUnmarshalCommandResult(t *testing.T) {
	result := CommandResult{
		Status: "ok",
		Data:   []byte{0xaa, 0xbb, 0xcc},
	}

	encoded, err := MarshalCommandResult(result)
	if err != nil {
		t.Fatalf("marshal command result: %v", err)
	}

	expected := []byte{
		0x02, 0x00,
		0x6f, 0x6b,
		0x03, 0x00, 0x00, 0x00,
		0xaa, 0xbb, 0xcc,
	}
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("unexpected command result bytes: got %x want %x", encoded, expected)
	}

	decoded, err := UnmarshalCommandResult(expected)
	if err != nil {
		t.Fatalf("unmarshal command result: %v", err)
	}
	if decoded.Status != result.Status || !bytes.Equal(decoded.Data, result.Data) {
		t.Fatalf("unexpected decoded result: got %+v want %+v", decoded, result)
	}
}
