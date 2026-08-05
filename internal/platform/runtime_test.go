package platform

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

type fakeFileInfo struct {
	name string
	size int64
	dir  bool
}

func (f fakeFileInfo) Name() string { return f.name }
func (f fakeFileInfo) Size() int64  { return f.size }
func (f fakeFileInfo) IsDir() bool  { return f.dir }

type fakeFileSystem struct {
	files map[string][]byte
}

func (f *fakeFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (f *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (f *fakeFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	f.files[path] = append([]byte(nil), data...)
	return nil
}

func (f *fakeFileSystem) Remove(path string) error {
	delete(f.files, path)
	return nil
}

func (f *fakeFileSystem) Stat(path string) (FileInfo, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return fakeFileInfo{name: path, size: int64(len(data))}, nil
}

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

type fakeProcess struct {
	args   []string
	env    map[string]string
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	exit   int
}

func (f *fakeProcess) Args() []string {
	return append([]string(nil), f.args...)
}

func (f *fakeProcess) Getenv(key string) string {
	return f.env[key]
}

func (f *fakeProcess) Stdout() io.Writer {
	return f.stdout
}

func (f *fakeProcess) Stderr() io.Writer {
	return f.stderr
}

func (f *fakeProcess) Exit(code int) {
	f.exit = code
}

type fakeTerminal struct {
	title string
	err   error
}

func (f *fakeTerminal) SetTitle(title string) error {
	if f.err != nil {
		return f.err
	}
	f.title = title
	return nil
}

func TestRuntimeSupportsInMemoryFileSystem(t *testing.T) {
	fs := &fakeFileSystem{files: map[string][]byte{}}
	runtime := Runtime{FileSystem: fs}

	if err := runtime.FileSystem.WriteFile("state/map.drmap", []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	data, err := runtime.FileSystem.ReadFile("state/map.drmap")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestRuntimeClockAndProcessAreDeterministic(t *testing.T) {
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	process := &fakeProcess{
		args:   []string{"discrescue", "--worker"},
		env:    map[string]string{workerModeEnvTestKey: workerModeEnvTestValue},
		stdout: stdout,
		stderr: stderr,
	}
	runtime := Runtime{
		Clock:   fakeClock{now: now},
		Process: process,
	}

	if got := runtime.Clock.Now(); !got.Equal(now) {
		t.Fatalf("unexpected time: got %v want %v", got, now)
	}
	if got := runtime.Process.Getenv(workerModeEnvTestKey); got != workerModeEnvTestValue {
		t.Fatalf("unexpected env value: got %q want %q", got, workerModeEnvTestValue)
	}
	runtime.Process.Exit(3)
	if process.exit != 3 {
		t.Fatalf("unexpected exit code: got %d want 3", process.exit)
	}
}

func TestRuntimeTerminalReportsErrors(t *testing.T) {
	terminal := &fakeTerminal{err: errors.New("title failed")}
	runtime := Runtime{Terminal: terminal}

	if err := runtime.Terminal.SetTitle("DiscRescue"); err == nil {
		t.Fatal("expected terminal error")
	}
}

const workerModeEnvTestKey = "DISKRESCUE_MODE"
const workerModeEnvTestValue = "worker"
