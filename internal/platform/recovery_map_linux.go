//go:build linux

package platform

import (
	"discrescue/internal/recoverymap"
	"fmt"
	"os"
	"path/filepath"
)

func openLinuxRecoveryMap(input RecoveryInput) (*recoverymap.Store, bool, error) {
	status, err := (OSRecovery{}).InspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	if status.CanStartNew {
		transaction := &recoverymap.StartupTransaction{}
		if dir := filepath.Dir(input.OutputPath); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, false, err
			}
		}
		output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
		if err != nil {
			return nil, false, fmt.Errorf("prepare Linux output %s: %w", input.OutputPath, err)
		}
		if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
			_ = output.Close()
			return nil, false, err
		}
		if err := output.Close(); err != nil {
			return nil, false, err
		}
		transaction.TrackCreated(input.OutputPath)
		header, err := recoveryMapHeader(input)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		state, err := recoverymap.Create(status.MapPath, header)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		transaction.TrackCreated(status.MapPath)
		transaction.Commit()
		return state, false, nil
	}
	if status.CanResume {
		state, err := recoverymap.Open(status.MapPath, recoverymap.Geometry{LogicalSectorSize: input.LogicalSectorSize, ExpectedSectorCount: input.CapacitySectors})
		return state, true, err
	}
	return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
}

func linuxRecoveryMapPath(outputPath string) string {
	if filepath.Ext(outputPath) == ".iso" {
		return outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".drmap"
	}
	return outputPath + ".drmap"
}
