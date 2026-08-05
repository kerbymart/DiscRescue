package main

import (
	"bytes"
	"testing"

	"discrescue/internal/device"
)

func TestRunWorkerWritesHelloFrame(t *testing.T) {
	var output bytes.Buffer

	if err := runWorker(&output); err != nil {
		t.Fatalf("run worker: %v", err)
	}

	frame, err := device.UnmarshalFrame(output.Bytes())
	if err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if frame.Type != device.MessageHello {
		t.Fatalf("unexpected frame type: %v", frame.Type)
	}
	if frame.RequestID != 1 {
		t.Fatalf("unexpected request id: %d", frame.RequestID)
	}

	hello, err := device.UnmarshalWorkerHello(frame.Payload)
	if err != nil {
		t.Fatalf("unmarshal worker hello: %v", err)
	}
	if hello.ProtocolVersion != device.ProtocolVersion || hello.WorkerID != "worker-bootstrap" {
		t.Fatalf("unexpected worker hello: %+v", hello)
	}
}
