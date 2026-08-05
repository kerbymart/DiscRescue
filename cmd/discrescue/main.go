package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if isWorkerMode(os.Args, os.Getenv(workerModeEnv)) {
		return runWorker(os.Stdout)
	}

	program := tea.NewProgram(app.NewModel())
	_, err := program.Run()
	return err
}
