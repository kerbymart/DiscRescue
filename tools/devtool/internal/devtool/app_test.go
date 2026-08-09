package devtool

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type recordingRunner struct {
	commands []Command
	err      error
}

func (r *recordingRunner) Run(_ context.Context, command Command) error {
	r.commands = append(r.commands, command)
	return r.err
}

func TestRunBuildConstructsShellFreeCommand(t *testing.T) {
	runner := &recordingRunner{}
	root := t.TempDir()
	app := App{Root: root, Runner: runner, Out: io.Discard, ErrOut: io.Discard}
	if err := app.Run(context.Background(), []string{"build", "--version", "1.2.3", "--commit", "abc", "--build-date", "today", "--output", filepath.Join("nested dir", "discrescue")}); err != nil {
		t.Fatalf("run build: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(runner.commands))
	}
	want := []string{"build", "-trimpath", "-ldflags", "-X discrescue/internal/buildinfo.Version=1.2.3 -X discrescue/internal/buildinfo.Commit=abc -X discrescue/internal/buildinfo.BuildDate=today", "-o", filepath.Join("nested dir", "discrescue"), "./cmd/discrescue"}
	if got := runner.commands[0]; got.Name != "go" || !reflect.DeepEqual(got.Args, want) || got.Dir != root {
		t.Fatalf("command = %#v, want go command in %q with args %#v", got, root, want)
	}
}

func TestRunPropagatesRunnerFailure(t *testing.T) {
	wantErr := errors.New("boom")
	runner := &recordingRunner{err: wantErr}
	app := App{Root: t.TempDir(), Runner: runner, Out: io.Discard, ErrOut: io.Discard}
	err := app.Run(context.Background(), []string{"test"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestGoFilesAreSortedAndExcludeVendor(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"z.go", "a.go", filepath.Join("vendor", "ignored.go")} {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := goFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go", "z.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
}
