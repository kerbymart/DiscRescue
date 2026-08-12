//go:build linux

package platform

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"discrescue/internal/recovery"
)

func openLinuxSource(devicePath string) (*recovery.ReopenableReaderAt, error) {
	sourceFile, err := os.OpenFile(devicePath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open Linux optical source %s read-only: %w", devicePath, err)
	}
	source, err := recovery.NewReopenableReaderAt(sourceFile, sourceFile.Close, func() (io.ReaderAt, error) {
		return os.OpenFile(devicePath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	})
	if err != nil {
		_ = sourceFile.Close()
		return nil, fmt.Errorf("prepare Linux optical source %s: %w", devicePath, err)
	}
	return source, nil
}
