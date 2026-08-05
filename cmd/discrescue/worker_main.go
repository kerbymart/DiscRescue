package main

import (
	"io"
	"slices"

	"discrescue/internal/device"
)

const workerModeEnv = "DISKRESCUE_MODE"
const workerModeValue = "worker"

func isWorkerMode(args []string, mode string) bool {
	return mode == workerModeValue || slices.Contains(args[1:], "--worker")
}

func runWorker(out io.Writer) error {
	helloPayload, err := device.MarshalWorkerHello(device.WorkerHello{
		ProtocolVersion: device.ProtocolVersion,
		WorkerID:        "worker-bootstrap",
	})
	if err != nil {
		return err
	}

	frame, err := device.MarshalFrame(device.Frame{
		Type:      device.MessageHello,
		RequestID: 1,
		Payload:   helloPayload,
	})
	if err != nil {
		return err
	}

	_, err = out.Write(frame)
	return err
}
