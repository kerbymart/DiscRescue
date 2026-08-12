package devtool

import (
	"context"
	"flag"
	"fmt"
	"io"
)

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
