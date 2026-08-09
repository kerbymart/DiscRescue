package devtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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

func (a App) runCheck(ctx context.Context) error {
	steps := []struct {
		name string
		args []string
	}{
		{"format", []string{"format", "--check"}},
		{"vet", []string{"vet", "./..."}},
		{"test", []string{"test", "./..."}},
		{"build", []string{"build", "-trimpath", "./cmd/discrescue"}},
	}
	for _, step := range steps {
		if step.args[0] == "format" {
			if err := a.runFormat(ctx, step.args[1:]); err != nil {
				return err
			}
			continue
		}
		if err := a.runNamed(ctx, step.name, Command{Name: "go", Args: step.args, Dir: a.Root, Env: defaultEnv()}); err != nil {
			return err
		}
	}
	return nil
}

func (a App) runRelease(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("release", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	raceMode := flags.String("race", "auto", "race policy: auto, on, or off")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *raceMode != "auto" && *raceMode != "on" && *raceMode != "off" {
		return fmt.Errorf("invalid --race=%q: want auto, on, or off", *raceMode)
	}
	steps := []struct {
		name string
		args []string
	}{
		{"format", nil},
		{"test", []string{"test", "./..."}},
		{"vet", []string{"vet", "./..."}},
		{"build", []string{"build", "-trimpath", "./cmd/discrescue"}},
		{"command audit", []string{"test", "./internal/testdevice", "-run", "TestRequestAudit"}},
		{"simulator integration", []string{"test", "./internal/testdevice", "-run", "TestScenario"}},
		{"soak and leak", []string{"test", "./internal/testdevice", "-run", "TestScenarioValidation(Soak|NoGoroutineLeak)"}},
	}
	for _, step := range steps {
		if step.name == "format" {
			if err := a.runFormat(ctx, []string{"--check"}); err != nil {
				return err
			}
			continue
		}
		if err := a.runNamed(ctx, step.name, Command{Name: "go", Args: step.args, Dir: a.Root, Env: defaultEnv()}); err != nil {
			return err
		}
	}
	if *raceMode == "off" || (*raceMode == "auto" && !raceSupported()) {
		fmt.Fprintln(a.Out, "[devtool] race: SKIPPED (unsupported platform or CGO is unavailable)")
	} else if err := a.runNamed(ctx, "race", Command{Name: "go", Args: []string{"test", "-race", "./..."}, Dir: a.Root, Env: defaultEnv()}); err != nil {
		return err
	}
	return a.runNamed(ctx, "throughput and CPU benchmarks", Command{Name: "go", Args: []string{"test", "-run", "^$", "-bench", "Benchmark(BuildPlan|VerifyExternal|VerifyImage)", "./internal/merge", "./internal/integrity"}, Dir: a.Root, Env: defaultEnv()})
}

func (a App) runBuild(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	version := flags.String("version", "dev", "embedded version")
	commit := flags.String("commit", "unknown", "embedded commit")
	buildDate := flags.String("build-date", "unknown", "embedded build date")
	output := flags.String("output", filepath.Join("dist", "discrescue"), "output executable path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *output == "" {
		return errors.New("build: --output must not be empty")
	}
	if dir := filepath.Dir(filepath.Join(a.Root, *output)); dir != a.Root {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create build output directory: %w", err)
		}
	}
	ldflags := fmt.Sprintf("-X discrescue/internal/buildinfo.Version=%s -X discrescue/internal/buildinfo.Commit=%s -X discrescue/internal/buildinfo.BuildDate=%s", *version, *commit, *buildDate)
	return a.runNamed(ctx, "build", Command{Name: "go", Args: []string{"build", "-trimpath", "-ldflags", ldflags, "-o", *output, "./cmd/discrescue"}, Dir: a.Root, Env: defaultEnv()})
}

func (a App) runNamed(ctx context.Context, name string, command Command) error {
	fmt.Fprintf(a.Out, "[devtool] %s: running\n", name)
	if err := a.Runner.Run(ctx, command); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find repository root (go.mod)")
		}
		dir = parent
	}
}

func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == ".git" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func raceSupported() bool {
	if os.Getenv("CGO_ENABLED") == "0" {
		return false
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return false
	}
	return runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
}
