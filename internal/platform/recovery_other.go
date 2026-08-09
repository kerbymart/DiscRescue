//go:build !windows && !darwin && !linux

package platform

import "fmt"

type OSRecovery struct{}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	return nil, fmt.Errorf("real image recovery is not implemented for this platform")
}

func (OSRecovery) InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: real image recovery is not implemented for this platform")
}
