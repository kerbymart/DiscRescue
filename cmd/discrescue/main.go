package main

import (
	"fmt"
	"io"

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

	program := newProgram(app.Services{Optical: runtime.Optical, Recovery: runtime.Recovery}, runtime.Process.Stdout())
	_, err := program.Run()
	return err
}

func newProgram(services app.Services, output io.Writer, opts ...tea.ProgramOption) *tea.Program {
	base := []tea.ProgramOption{
		tea.WithOutput(output),
	}
	base = append(base, opts...)
	return tea.NewProgram(app.NewRecoveryProgramModel(services), base...)
}
