package devtool

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Command is a shell-free process description.
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

// ExecRunner runs commands directly and streams their output.
type ExecRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r ExecRunner) Run(ctx context.Context, command Command) error {
	if command.Name == "" {
		return fmt.Errorf("run command: executable is required")
	}
	process := exec.CommandContext(ctx, command.Name, command.Args...)
	process.Dir = command.Dir
	process.Env = command.Env
	process.Stdout = r.Stdout
	process.Stderr = r.Stderr
	if err := process.Run(); err != nil {
		return fmt.Errorf("run %s: %w", command.Name, err)
	}
	return nil
}

func defaultEnv() []string { return os.Environ() }
