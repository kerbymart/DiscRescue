//go:build windows

package platform

import (
	"fmt"

	"discrescue/internal/recoverymap"
)

func openRecoveryMapState(input RecoveryInput) (*recoverymap.Store, bool, error) {
	status, err := inspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	switch {
	case status.CanStartNew:
		mapPath := recoveryMapPath(input.OutputPath)
		if err := preflightWindowsSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		transaction := &recoverymap.StartupTransaction{}
		if err := prepareWindowsOutput(input, transaction); err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		state, resumed, err := createRecoveryMapState(input, mapPath)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		transaction.TrackCreated(mapPath)
		transaction.Commit()
		return state, resumed, nil
	case status.CanResume:
		if err := preflightWindowsSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		return loadRecoveryMapState(input, status.MapPath)
	default:
		return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
	}
}

func createRecoveryMapState(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	header, err := recoveryMapHeader(input)
	if err != nil {
		return nil, false, err
	}
	store, err := recoverymap.Create(mapPath, header)
	return store, false, err
}

func loadRecoveryMapState(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	store, err := recoverymap.Open(mapPath, recoverymap.Geometry{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
	})
	return store, true, err
}
