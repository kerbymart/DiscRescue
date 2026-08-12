package devtool

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// App implements the repository validation and build commands.
type App struct {
	Root   string
	Runner Runner
	Out    io.Writer
	ErrOut io.Writer
}

// Main runs the devtool CLI and returns a process exit status.
func Main(ctx context.Context, args []string, out, errOut io.Writer) int {
	root, err := findRoot()
	if err != nil {
		fmt.Fprintln(errOut, "[devtool]", err)
		return 1
	}
	app := App{Root: root, Runner: ExecRunner{Stdout: out, Stderr: errOut}, Out: out, ErrOut: errOut}
	if err := app.Run(ctx, args); err != nil {
		fmt.Fprintln(errOut, "[devtool] FAIL:", err)
		return 1
	}
	return 0
}

// Run dispatches one devtool command.
func (a App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: devtool <format|test|check|release|build>")
	}
	switch args[0] {
	case "format":
		return a.runFormat(ctx, args[1:])
	case "test":
		return a.runNamed(ctx, "test", Command{Name: "go", Args: []string{"test", "./..."}})
	case "check":
		return a.runCheck(ctx)
	case "release":
		return a.runRelease(ctx, args[1:])
	case "build":
		return a.runBuild(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
