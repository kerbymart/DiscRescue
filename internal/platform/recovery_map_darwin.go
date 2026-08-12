//go:build darwin

package platform

import (
	"discrescue/internal/recoverymap"
	"fmt"
	"strings"
)

func openDarwinRecoveryMap(input RecoveryInput) (*recoverymap.Store, bool, error) {
	status, err := (OSRecovery{}).InspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	if status.CanStartNew {
		if err := preflightDarwinSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		transaction := &recoverymap.StartupTransaction{}
		if err := prepareDarwinOutput(input, transaction); err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		state, resumed, err := createDarwinRecoveryMap(input, status.MapPath)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		transaction.TrackCreated(status.MapPath)
		transaction.Commit()
		return state, resumed, nil
	}
	if status.CanResume {
		if err := preflightDarwinSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		return loadDarwinRecoveryMap(input, status.MapPath)
	}
	return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
}

func createDarwinRecoveryMap(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	header, err := recoveryMapHeader(input)
	if err != nil {
		return nil, false, err
	}
	store, err := recoverymap.Create(mapPath, header)
	return store, false, err
}

func loadDarwinRecoveryMap(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	store, err := recoverymap.Open(mapPath, recoverymap.Geometry{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
	})
	return store, true, err
}

func darwinRecoveryMapPath(outputPath string) string {
	if strings.HasSuffix(outputPath, ".iso") {
		return strings.TrimSuffix(outputPath, ".iso") + ".drmap"
	}
	return outputPath + ".drmap"
}
