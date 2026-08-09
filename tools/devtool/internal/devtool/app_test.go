package devtool

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type recordedCommand struct {
	name string
	args []string
	dir  string
}

type fakeRunner struct {
	commands []recordedCommand
	outputs  [][]byte
	err      error
}

func (f *fakeRunner) Run(_ context.Context, command Command) error {
	f.commands = append(f.commands, recordedCommand{command.Name, append([]string(nil), command.Args...), command.Dir})
	return f.err
}

func (f *fakeRunner) Output(_ context.Context, command Command) ([]byte, error) {
	f.commands = append(f.commands, recordedCommand{command.Name, append([]string(nil), command.Args...), command.Dir})
	if len(f.outputs) == 0 {
		return nil, f.err
	}
	output := f.outputs[0]
	f.outputs = f.outputs[1:]
	return output, f.err
}

func TestCheckCommandOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	app := newApp(root, runner, io.Discard, io.Discard)
	if err := app.Run(context.Background(), []string{"check"}); err != nil {
		t.Fatal(err)
	}
	got := make([][]string, len(runner.commands))
	for i, command := range runner.commands {
		got[i] = append([]string{command.name}, command.args...)
	}
	want := [][]string{
		{"gofmt", "-l", filepath.Join(root, "main.go")},
		{"go", "vet", "./..."},
		{"go", "test", "./..."},
		{"go", "build", "-trimpath", "./cmd/discrescue"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestFormatCheckRejectsUnformattedFiles(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{outputs: [][]byte{[]byte(file + "\n")}}
	app := newApp(root, runner, io.Discard, io.Discard)
	err := app.Run(context.Background(), []string{"format", "--check"})
	if err == nil || !strings.Contains(err.Error(), "files require formatting") {
		t.Fatalf("error = %v, want formatting failure", err)
	}
}

func TestBuildKeepsOutputPathAsOneArgument(t *testing.T) {
	root := t.TempDir()
	runner := &fakeRunner{}
	app := newApp(root, runner, io.Discard, io.Discard)
	if err := app.Run(context.Background(), []string{"build", "--output", "dist/Disc Rescue"}); err != nil {
		t.Fatal(err)
	}
	got := runner.commands[0]
	if got.args[gotArgIndex(got.args, "-o")+1] != filepath.Join(root, "dist/Disc Rescue") {
		t.Fatalf("build args = %#v", got.args)
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); err != nil {
		t.Fatalf("output directory was not created: %v", err)
	}
}

func TestBuildPropagatesRunnerFailure(t *testing.T) {
	runner := &fakeRunner{err: errors.New("failed")}
	app := newApp(t.TempDir(), runner, io.Discard, io.Discard)
	if err := app.Run(context.Background(), []string{"test"}); err == nil {
		t.Fatal("expected failure")
	}
}

func TestBuildRejectsShellLikeMetadata(t *testing.T) {
	runner := &fakeRunner{}
	app := newApp(t.TempDir(), runner, io.Discard, io.Discard)
	if err := app.Run(context.Background(), []string{"build", "--version", "dev;touch"}); err == nil {
		t.Fatal("expected unsafe metadata to be rejected")
	}
	if len(runner.commands) != 0 {
		t.Fatalf("commands = %#v, want none", runner.commands)
	}
}

func TestGoFilesAreSortedAndSkipBuildOutput(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "z.go"), filepath.Join(root, "a.go"), filepath.Join(root, "dist", "generated.go")}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := goFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "a.go"), filepath.Join(root, "z.go")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
}

func gotArgIndex(args []string, value string) int {
	for i, arg := range args {
		if arg == value {
			return i
		}
	}
	return -1
}
