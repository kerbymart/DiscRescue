package devtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
