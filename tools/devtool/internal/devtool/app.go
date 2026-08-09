package devtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// App dispatches the repository's shell-neutral development commands.
type App struct {
	stdout io.Writer
	stderr io.Writer
	runner Runner
	root   string
}

// New creates a development-tool application rooted at the current directory.
func New(stdout, stderr io.Writer) *App {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}
	return &App{
		stdout: stdout,
		stderr: stderr,
		runner: ExecRunner{Stdout: stdout, Stderr: stderr, Stdin: os.Stdin},
		root:   root,
	}
}

func newApp(root string, runner Runner, stdout, stderr io.Writer) *App {
	return &App{root: root, runner: runner, stdout: stdout, stderr: stderr}
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("devtool: command required (format, test, check, release, or build)")
	}

	switch args[0] {
	case "format":
		return a.runFormat(ctx, args[1:])
	case "test":
		if len(args) != 1 {
			return errors.New("devtool: test does not accept arguments")
		}
		return a.runTest(ctx)
	case "check":
		if len(args) != 1 {
			return errors.New("devtool: check does not accept arguments")
		}
		return a.runCheck(ctx)
	case "release":
		return a.runRelease(ctx, args[1:])
	case "build":
		return a.runBuild(ctx, args[1:])
	default:
		return fmt.Errorf("devtool: unknown command %q", args[0])
	}
}

func (a *App) runFormat(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("format", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	check := flags.Bool("check", false, "verify formatting without modifying files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("devtool: format does not accept positional arguments")
	}

	files, err := goFiles(a.root)
	if err != nil {
		return fmt.Errorf("devtool: find Go files: %w", err)
	}
	if len(files) == 0 {
		a.log("format: no Go files found")
		return nil
	}

	argsForGofmt := []string{"-w"}
	if *check {
		argsForGofmt = []string{"-l"}
	}
	argsForGofmt = append(argsForGofmt, files...)
	if *check {
		a.log("format: checking")
		output, err := runOutput(ctx, a.runner, Command{Name: "gofmt", Args: argsForGofmt, Dir: a.root})
		if err != nil {
			return fmt.Errorf("devtool: format: %w", err)
		}
		if len(strings.TrimSpace(string(output))) != 0 {
			return fmt.Errorf("devtool: format: files require formatting:\n%s", strings.TrimSpace(string(output)))
		}
		return nil
	}

	a.log("format: applying")
	if err := a.runner.Run(ctx, Command{Name: "gofmt", Args: argsForGofmt, Dir: a.root}); err != nil {
		return fmt.Errorf("devtool: format: %w", err)
	}
	return nil
}

func (a *App) runTest(ctx context.Context) error {
	a.log("test: running")
	return a.runGo(ctx, "test", "./...")
}

func (a *App) runCheck(ctx context.Context) error {
	if err := a.runFormat(ctx, []string{"--check"}); err != nil {
		return err
	}
	a.log("vet: running")
	if err := a.runGo(ctx, "vet", "./..."); err != nil {
		return err
	}
	if err := a.runTest(ctx); err != nil {
		return err
	}
	a.log("build: running")
	return a.runGo(ctx, "build", "-trimpath", "./cmd/discrescue")
}

func (a *App) runGo(ctx context.Context, args ...string) error {
	if err := a.runner.Run(ctx, Command{Name: "go", Args: args, Dir: a.root}); err != nil {
		return fmt.Errorf("devtool: go %s: %w", args[0], err)
	}
	return nil
}

func goFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "dist") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func (a *App) log(format string, args ...any) {
	if a.stdout != nil {
		fmt.Fprintf(a.stdout, "[devtool] "+format+"\n", args...)
	}
}

func environmentLine() string {
	return fmt.Sprintf("platform: %s/%s", runtime.GOOS, runtime.GOARCH)
}
