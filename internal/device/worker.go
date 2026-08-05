package device

import (
	"encoding/binary"
	"fmt"
)

type WorkerHello struct {
	ProtocolVersion uint16
	WorkerID        string
}

type WorkerHelloAck struct {
	ProtocolVersion uint16
	Accepted        bool
}

func (h WorkerHello) Validate() error {
	if h.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("validate worker hello: unsupported protocol version %d", h.ProtocolVersion)
	}
	if h.WorkerID == "" {
		return fmt.Errorf("validate worker hello: worker id is required")
	}
	if len(h.WorkerID) > 0xffff {
		return fmt.Errorf("validate worker hello: worker id length %d exceeds uint16", len(h.WorkerID))
	}
	return nil
}

func MarshalWorkerHello(hello WorkerHello) ([]byte, error) {
	if err := hello.Validate(); err != nil {
		return nil, err
	}

	encoded := make([]byte, 4+len(hello.WorkerID))
	binary.LittleEndian.PutUint16(encoded[0:2], hello.ProtocolVersion)
	binary.LittleEndian.PutUint16(encoded[2:4], uint16(len(hello.WorkerID)))
	copy(encoded[4:], []byte(hello.WorkerID))
	return encoded, nil
}

func UnmarshalWorkerHello(payload []byte) (WorkerHello, error) {
	if len(payload) < 4 {
		return WorkerHello{}, fmt.Errorf("unmarshal worker hello: payload too short")
	}
	idLength := int(binary.LittleEndian.Uint16(payload[2:4]))
	if len(payload) != 4+idLength {
		return WorkerHello{}, fmt.Errorf("unmarshal worker hello: payload length %d does not match expected %d", len(payload), 4+idLength)
	}
	hello := WorkerHello{
		ProtocolVersion: binary.LittleEndian.Uint16(payload[0:2]),
		WorkerID:        string(payload[4:]),
	}
	return hello, hello.Validate()
}

func (ack WorkerHelloAck) Validate() error {
	if ack.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("validate worker hello ack: unsupported protocol version %d", ack.ProtocolVersion)
	}
	return nil
}

func MarshalWorkerHelloAck(ack WorkerHelloAck) ([]byte, error) {
	if err := ack.Validate(); err != nil {
		return nil, err
	}

	encoded := make([]byte, 3)
	binary.LittleEndian.PutUint16(encoded[0:2], ack.ProtocolVersion)
	if ack.Accepted {
		encoded[2] = 1
	}
	return encoded, nil
}

func UnmarshalWorkerHelloAck(payload []byte) (WorkerHelloAck, error) {
	if len(payload) != 3 {
		return WorkerHelloAck{}, fmt.Errorf("unmarshal worker hello ack: expected 3 bytes, got %d", len(payload))
	}
	ack := WorkerHelloAck{
		ProtocolVersion: binary.LittleEndian.Uint16(payload[0:2]),
		Accepted:        payload[2] == 1,
	}
	return ack, ack.Validate()
}
