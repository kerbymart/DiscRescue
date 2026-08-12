//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"

	"discrescue/internal/recoverymap"
)

func prepareWindowsOutput(input RecoveryInput, transaction *recoverymap.StartupTransaction) error {
	if dir := filepath.Dir(input.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("create output image %s: %w", input.OutputPath, err)
	}
	transaction.TrackCreated(input.OutputPath)
	totalBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	if err := output.Truncate(int64(totalBytes)); err != nil {
		_ = output.Close()
		return fmt.Errorf("prepare output image %s: %w", input.OutputPath, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close prepared output image %s: %w", input.OutputPath, err)
	}
	return nil
}
