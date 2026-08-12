//go:build windows

package platform

import (
	"fmt"
	"os"
)

func preflightWindowsSource(devicePath string) error {
	rawPath := rawVolumePath(devicePath)
	source, err := os.Open(rawPath)
	if err != nil {
		return fmt.Errorf("preflight source volume %s: %w", rawPath, err)
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("preflight source volume %s: close: %w", rawPath, err)
	}
	return nil
}
