//go:build darwin

package platform

import (
	"fmt"
	"os"
	"path/filepath"

	"discrescue/internal/recoverymap"
)

func openDarwinOutput(input RecoveryInput, startup *recoverymap.StartupTransaction) (*os.File, error) {
	if dir := filepath.Dir(input.OutputPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	flags := os.O_RDWR | os.O_CREATE
	if startup != nil {
		flags |= os.O_EXCL
	}
	output, err := os.OpenFile(input.OutputPath, flags, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create output image %s: %w", input.OutputPath, err)
	}
	if startup != nil {
		startup.TrackCreated(input.OutputPath)
	}
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		_ = output.Close()
		return nil, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err)
	}
	if startup != nil {
		startup.Commit()
	}
	return output, nil
}

func prepareDarwinOutput(input RecoveryInput, transaction *recoverymap.StartupTransaction) error {
	if dir := filepath.Dir(input.OutputPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("create output image %s: %w", input.OutputPath, err)
	}
	transaction.TrackCreated(input.OutputPath)
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		_ = output.Close()
		return fmt.Errorf("prepare output image %s: %w", input.OutputPath, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close prepared output image %s: %w", input.OutputPath, err)
	}
	return nil
}
