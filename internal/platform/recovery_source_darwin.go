//go:build darwin

package platform

import (
	"fmt"
	"io"
	"os"

	"discrescue/internal/recovery"
)

func openDarwinSource(devicePath string) (*recovery.ReopenableReaderAt, error) {
	rawPath, err := normalizeDarwinOpticalDevice(devicePath)
	if err != nil {
		return nil, fmt.Errorf("open macOS optical source: %w", err)
	}
	sourceFile, err := os.Open(rawPath)
	if err != nil {
		return nil, fmt.Errorf("open macOS optical source %s read-only: %w", rawPath, err)
	}
	source, err := recovery.NewReopenableReaderAt(sourceFile, sourceFile.Close, func() (io.ReaderAt, error) {
		return os.Open(rawPath)
	})
	if err != nil {
		_ = sourceFile.Close()
		return nil, fmt.Errorf("prepare macOS optical source %s: %w", rawPath, err)
	}
	return source, nil
}

func preflightDarwinSource(devicePath string) error {
	rawPath, err := normalizeDarwinOpticalDevice(devicePath)
	if err != nil {
		return fmt.Errorf("preflight macOS optical source: %w", err)
	}
	source, err := os.Open(rawPath)
	if err != nil {
		return fmt.Errorf("preflight macOS optical source %s read-only: %w", rawPath, err)
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("preflight macOS optical source %s: close: %w", rawPath, err)
	}
	return nil
}
