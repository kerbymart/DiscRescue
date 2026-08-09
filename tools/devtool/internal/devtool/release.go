package devtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
)

func (a *App) runRelease(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("release", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	race := flags.String("race", "auto", "race policy: auto, on, or off")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("devtool: release does not accept positional arguments")
	}
	if *race != "auto" && *race != "on" && *race != "off" {
		return fmt.Errorf("devtool: invalid race mode %q", *race)
	}

	a.log(environmentLine())
	if err := a.runFormat(ctx, []string{"--check"}); err != nil {
		return err
	}
	if err := a.runTest(ctx); err != nil {
		return err
	}
	a.log("vet: running")
	if err := a.runGo(ctx, "vet", "./..."); err != nil {
		return err
	}
	a.log("build: running")
	if err := a.runGo(ctx, "build", "-trimpath", "./cmd/discrescue"); err != nil {
		return err
	}
	for _, gate := range []struct {
		name string
		args []string
	}{
		{"command audit", []string{"test", "./internal/testdevice", "-run", "TestRequestAudit"}},
		{"simulator integration", []string{"test", "./internal/testdevice", "-run", "TestScenario"}},
		{"soak and leak", []string{"test", "./internal/testdevice", "-run", "TestScenarioValidation(Soak|NoGoroutineLeak)"}},
	} {
		a.log("%s: running", gate.name)
		if err := a.runGo(ctx, gate.args...); err != nil {
			return err
		}
	}

	switch *race {
	case "off":
		a.log("race: SKIPPED (disabled by flag)")
	case "on":
		a.log("race: running")
		if err := a.runGo(ctx, "test", "-race", "./..."); err != nil {
			return err
		}
	default:
		cgoEnabled, err := a.cgoEnabled(ctx)
		if err != nil {
			a.log("race: SKIPPED (%v)", err)
		} else if !cgoEnabled {
			a.log("race: SKIPPED (CGO_ENABLED=0)")
		} else {
			a.log("race: running")
			if err := a.runGo(ctx, "test", "-race", "./..."); err != nil {
				return err
			}
		}
	}

	a.log("throughput and cpu benchmarks: running")
	return a.runGo(ctx, "test", "-run", "^$", "-bench", "Benchmark(BuildPlan|VerifyExternal|VerifyImage)", "./internal/merge", "./internal/integrity")
}

func (a *App) cgoEnabled(ctx context.Context) (bool, error) {
	output, err := runOutput(ctx, a.runner, Command{Name: "go", Args: []string{"env", "CGO_ENABLED"}, Dir: a.root})
	if err != nil {
		return false, fmt.Errorf("unable to query Go CGO_ENABLED: %w", err)
	}
	switch string(output) {
	case "1\n", "1\r\n", "1":
		return true, nil
	case "0\n", "0\r\n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("Go reported unexpected CGO_ENABLED value %q", string(output))
	}
}
