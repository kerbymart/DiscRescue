package devtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) runBuild(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	version := flags.String("version", "dev", "version embedded in the executable")
	commit := flags.String("commit", "unknown", "commit embedded in the executable")
	buildDate := flags.String("build-date", "unknown", "build date embedded in the executable")
	output := flags.String("output", filepath.Join("dist", "discrescue"), "output executable path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("devtool: build does not accept positional arguments")
	}
	if *output == "" {
		return errors.New("devtool: build output cannot be empty")
	}
	for name, value := range map[string]string{"version": *version, "commit": *commit, "build-date": *buildDate} {
		if !safeMetadata(value) {
			return fmt.Errorf("devtool: build %s contains unsupported characters", name)
		}
	}

	outputPath := *output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(a.root, outputPath)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("devtool: create build output directory: %w", err)
	}
	ldflags := []string{
		"-X", "discrescue/internal/buildinfo.Version=" + *version,
		"-X", "discrescue/internal/buildinfo.Commit=" + *commit,
		"-X", "discrescue/internal/buildinfo.BuildDate=" + *buildDate,
	}
	a.log("build: running")
	if err := a.runner.Run(ctx, Command{
		Name: "go",
		Args: []string{"build", "-trimpath", "-ldflags", joinArgs(ldflags), "-o", outputPath, "./cmd/discrescue"},
		Dir:  a.root,
	}); err != nil {
		return fmt.Errorf("devtool: go build: %w", err)
	}
	return nil
}

func safeMetadata(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._-+/", character) {
			continue
		}
		return false
	}
	return true
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}
