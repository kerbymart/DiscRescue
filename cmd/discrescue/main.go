package main

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/app"
	"discrescue/internal/platform"
)

func main() {
	runtime := platform.NewRuntime()
	if err := run(runtime); err != nil {
		fmt.Fprintln(runtime.Process.Stderr(), err)
		runtime.Process.Exit(1)
	}
}

func run(runtime platform.Runtime) error {
	if isWorkerMode(runtime.Process.Args(), runtime.Process.Getenv(workerModeEnv)) {
		return runWorker(runtime.Process.Stdout())
	}

	if err := runtime.Terminal.SetTitle("DiscRescue"); err != nil {
		return err
	}

	program := tea.NewProgram(app.NewModel())
	_, err := program.Run()
	return err
}
