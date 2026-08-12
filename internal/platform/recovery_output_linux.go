//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"

	"discrescue/internal/recoverymap"
)

func openLinuxOutput(input RecoveryInput, startup *recoverymap.StartupTransaction) (*os.File, error) {
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
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		_ = output.Close()
		return nil, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err)
	}
	if startup != nil {
		startup.TrackCreated(input.OutputPath)
		startup.Commit()
	}
	return output, nil
}
