package device

import "testing"

func TestWorkerHelloRoundTrip(t *testing.T) {
	encoded, err := MarshalWorkerHello(WorkerHello{
		ProtocolVersion: ProtocolVersion,
		WorkerID:        "worker-01",
	})
	if err != nil {
		t.Fatalf("marshal worker hello: %v", err)
	}

	decoded, err := UnmarshalWorkerHello(encoded)
	if err != nil {
		t.Fatalf("unmarshal worker hello: %v", err)
	}
	if decoded.ProtocolVersion != ProtocolVersion || decoded.WorkerID != "worker-01" {
		t.Fatalf("unexpected decoded hello: %+v", decoded)
	}
}

func TestWorkerHelloRejectsWrongVersion(t *testing.T) {
	if _, err := MarshalWorkerHello(WorkerHello{
		ProtocolVersion: ProtocolVersion + 1,
		WorkerID:        "worker-01",
	}); err == nil {
		t.Fatal("expected wrong protocol version to fail")
	}
}

func TestWorkerHelloAckRoundTrip(t *testing.T) {
	encoded, err := MarshalWorkerHelloAck(WorkerHelloAck{
		ProtocolVersion: ProtocolVersion,
		Accepted:        true,
	})
	if err != nil {
		t.Fatalf("marshal worker hello ack: %v", err)
	}

	decoded, err := UnmarshalWorkerHelloAck(encoded)
	if err != nil {
		t.Fatalf("unmarshal worker hello ack: %v", err)
	}
	if !decoded.Accepted || decoded.ProtocolVersion != ProtocolVersion {
		t.Fatalf("unexpected decoded ack: %+v", decoded)
	}
}
