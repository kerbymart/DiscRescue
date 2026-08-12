package devtool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os/exec"
)

func (a App) runFormat(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("format", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	check := flags.Bool("check", false, "report files that need formatting without changing them")
	if err := flags.Parse(args); err != nil {
		return err
	}
	files, err := goFiles(a.Root)
	if err != nil {
		return fmt.Errorf("find Go files: %w", err)
	}
	if len(files) == 0 {
		fmt.Fprintln(a.Out, "[devtool] format: no Go files found")
		return nil
	}
	if *check {
		return a.checkFormat(ctx, files)
	}
	command := Command{Name: "gofmt", Args: append([]string{"-w"}, files...), Dir: a.Root, Env: defaultEnv()}
	return a.runNamed(ctx, "format", command)
}
func (a App) checkFormat(ctx context.Context, files []string) error {
	process := exec.CommandContext(ctx, "gofmt", append([]string{"-l"}, files...)...)
	process.Dir = a.Root
	process.Env = defaultEnv()
	output, err := process.Output()
	if err != nil {
		return fmt.Errorf("format check: %w", err)
	}
	if len(output) != 0 {
		return fmt.Errorf("format check: files need formatting:\n%s", output)
	}
	fmt.Fprintln(a.Out, "[devtool] format check: PASS")
	return nil
}
