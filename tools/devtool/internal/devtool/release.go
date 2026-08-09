package devtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
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
		if os.Getenv("CGO_ENABLED") == "0" {
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
