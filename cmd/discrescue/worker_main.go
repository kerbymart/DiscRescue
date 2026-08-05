package main

import (
	"fmt"
	"io"
	"slices"
)

const workerModeEnv = "DISKRESCUE_MODE"
const workerModeValue = "worker"

func isWorkerMode(args []string, mode string) bool {
	return mode == workerModeValue || slices.Contains(args[1:], "--worker")
}

func runWorker(out io.Writer) error {
	_, err := fmt.Fprintln(out, "discrescue worker mode is not implemented yet")
	return err
}
