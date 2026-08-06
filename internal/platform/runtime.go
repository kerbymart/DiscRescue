package platform

import (
	"io"
	"os"
	"time"
)

type FileInfo interface {
	Name() string
	Size() int64
	IsDir() bool
}

type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Remove(path string) error
	Stat(path string) (FileInfo, error)
}

type Clock interface {
	Now() time.Time
}

type Process interface {
	Args() []string
	Getenv(key string) string
	Stdout() io.Writer
	Stderr() io.Writer
	Exit(code int)
}

type Terminal interface {
	SetTitle(title string) error
}

type Runtime struct {
	FileSystem FileSystem
	Clock      Clock
	Process    Process
	Terminal   Terminal
	Optical    OpticalDiscovery
	Recovery   RecoveryService
}

func NewRuntime() Runtime {
	runtime := Runtime{
		FileSystem: OSFileSystem{},
		Clock:      SystemClock{},
		Process:    OSProcess{},
		Terminal:   OSTerminal{},
	}
	runtime.Optical = OSOpticalDiscovery{Process: runtime.Process}
	runtime.Recovery = OSRecovery{}
	return runtime
}

type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OSFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (OSFileSystem) Remove(path string) error {
	return os.Remove(path)
}

func (OSFileSystem) Stat(path string) (FileInfo, error) {
	return os.Stat(path)
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now()
}

type OSProcess struct{}

func (OSProcess) Args() []string {
	return os.Args
}

func (OSProcess) Getenv(key string) string {
	return os.Getenv(key)
}

func (OSProcess) Stdout() io.Writer {
	return os.Stdout
}

func (OSProcess) Stderr() io.Writer {
	return os.Stderr
}

func (OSProcess) Exit(code int) {
	os.Exit(code)
}

type OSTerminal struct{}

func (OSTerminal) SetTitle(title string) error {
	return os.Setenv("DISKRESCUE_WINDOW_TITLE", title)
}
