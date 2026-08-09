package devtool

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Command is a process invocation with arguments kept separate from the
// executable. It is deliberately not a shell command string.
type Command struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

// Runner executes a process description.
type Runner interface {
	Run(context.Context, Command) error
}

type outputRunner interface {
	Output(context.Context, Command) ([]byte, error)
}

// ExecRunner runs commands directly through os/exec. A nil Env inherits the
// current process environment.
type ExecRunner struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

func (r ExecRunner) Run(ctx context.Context, command Command) error {
	process := exec.CommandContext(ctx, command.Name, command.Args...)
	process.Dir = command.Dir
	process.Env = command.Env
	process.Stdout = r.Stdout
	process.Stderr = r.Stderr
	process.Stdin = r.Stdin
	return process.Run()
}

func (r ExecRunner) Output(ctx context.Context, command Command) ([]byte, error) {
	process := exec.CommandContext(ctx, command.Name, command.Args...)
	process.Dir = command.Dir
	process.Env = command.Env
	return process.Output()
}

func runOutput(ctx context.Context, runner Runner, command Command) ([]byte, error) {
	output, ok := runner.(outputRunner)
	if !ok {
		return nil, fmt.Errorf("devtool: runner cannot capture output for %s", command.Name)
	}
	return output.Output(ctx, command)
}
